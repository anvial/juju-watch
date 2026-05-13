package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/anvial/juju-watch/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/mattn/go-runewidth"
)

const sshCloseKey = tea.KeyCtrlCloseBracket

type sshPopup struct {
	request sshRequest
	runtime sshRuntime
	term    vt10x.Terminal
	status  string
}

type sshRequest struct {
	model        string
	machineID    string
	machineLabel string
	commandLine  string
	cols         int
	rows         int
}

type sshStartedMsg struct {
	request sshRequest
	runtime sshRuntime
	err     error
}

type sshOutputMsg struct {
	runtime sshRuntime
	data    []byte
}

type sshExitMsg struct {
	runtime sshRuntime
	err     error
}

type sshRuntime interface {
	io.Reader
	io.Writer
	io.Closer
	Resize(cols, rows int) error
}

type sshStarter interface {
	Start(req sshRequest) (sshRuntime, error)
}

type realSSHStarter struct{}

func (realSSHStarter) Start(req sshRequest) (sshRuntime, error) {
	cmd := exec.Command("juju", sshArgs(req.model, req.machineID)...)
	file, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(max(1, req.cols)),
		Rows: uint16(max(1, req.rows)),
	})
	if err != nil {
		return nil, err
	}
	return &ptySSHRuntime{file: file, cmd: cmd}, nil
}

type ptySSHRuntime struct {
	file *os.File
	cmd  *exec.Cmd
	once sync.Once
}

func (r *ptySSHRuntime) Read(p []byte) (int, error) {
	return r.file.Read(p)
}

func (r *ptySSHRuntime) Write(p []byte) (int, error) {
	return r.file.Write(p)
}

func (r *ptySSHRuntime) Resize(cols, rows int) error {
	return pty.Setsize(r.file, &pty.Winsize{
		Cols: uint16(max(1, cols)),
		Rows: uint16(max(1, rows)),
	})
}

func (r *ptySSHRuntime) Close() error {
	var err error
	r.once.Do(func() {
		err = r.file.Close()
		if r.cmd.Process != nil {
			_ = r.cmd.Process.Kill()
		}
		if waitErr := r.cmd.Wait(); err == nil && waitErr != nil && !errors.Is(waitErr, os.ErrProcessDone) {
			err = waitErr
		}
	})
	return err
}

func (m Model) openSSHForSelected() (tea.Model, tea.Cmd) {
	node, ok := m.graph.Nodes[m.selectedID]
	if !ok || node.Type != domain.NodeMachine {
		return m, nil
	}
	machineID, ok := jujuMachineID(node)
	if !ok {
		m.lastErr = fmt.Errorf("ssh machine id not found for %s", node.Label)
		m.addEvent(domain.Event{Kind: "ssh-error", Label: node.Label, ObjectID: node.ID, Message: m.lastErr.Error()})
		return m, nil
	}
	req := m.sshRequest(node, machineID)
	m.ssh = &sshPopup{
		request: req,
		term:    vt10x.New(vt10x.WithSize(req.cols, req.rows)),
		status:  "connecting",
	}
	return m, sshStartCmd(m.sshRun, req)
}

func (m Model) sshRequest(node domain.Node, machineID string) sshRequest {
	box := sshPopupBox(m.width, m.canvasHeight())
	cols := max(1, box.width-4)
	rows := max(1, box.height-4)
	return sshRequest{
		model:        m.cfg.Model,
		machineID:    machineID,
		machineLabel: node.Label,
		commandLine:  "juju " + strings.Join(sshArgs(m.cfg.Model, machineID), " "),
		cols:         cols,
		rows:         rows,
	}
}

func sshStartCmd(starter sshStarter, req sshRequest) tea.Cmd {
	return func() tea.Msg {
		if starter == nil {
			starter = realSSHStarter{}
		}
		runtime, err := starter.Start(req)
		return sshStartedMsg{request: req, runtime: runtime, err: err}
	}
}

func sshReadCmd(runtime sshRuntime) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := runtime.Read(buf)
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			return sshOutputMsg{runtime: runtime, data: data}
		}
		return sshExitMsg{runtime: runtime, err: err}
	}
}

func (m Model) handleSSHStarted(msg sshStartedMsg) (tea.Model, tea.Cmd) {
	if m.ssh == nil || m.ssh.request != msg.request {
		if msg.runtime != nil {
			_ = msg.runtime.Close()
		}
		return m, nil
	}
	if msg.err != nil {
		m.finishSSH(msg.err)
		return m, nil
	}
	m.ssh.runtime = msg.runtime
	m.ssh.status = "connected"
	return m, sshReadCmd(msg.runtime)
}

func (m Model) handleSSHOutput(msg sshOutputMsg) (tea.Model, tea.Cmd) {
	if m.ssh == nil || m.ssh.runtime != msg.runtime {
		return m, nil
	}
	_, _ = m.ssh.term.Write(msg.data)
	return m, sshReadCmd(msg.runtime)
}

func (m Model) handleSSHExit(msg sshExitMsg) (tea.Model, tea.Cmd) {
	if m.ssh == nil || m.ssh.runtime != msg.runtime {
		return m, nil
	}
	if normalSSHExit(msg.err) {
		m.finishSSH(nil)
		return m, nil
	}
	m.finishSSH(msg.err)
	return m, nil
}

func (m Model) handleSSHKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.ssh == nil {
		return m, nil
	}
	if tea.Key(msg).Type == sshCloseKey {
		m.finishSSH(nil)
		return m, nil
	}
	if m.ssh.runtime == nil {
		return m, nil
	}
	if data := sshKeyBytes(msg); len(data) > 0 {
		if _, err := m.ssh.runtime.Write(data); err != nil {
			m.finishSSH(err)
		}
	}
	return m, nil
}

func (m *Model) finishSSH(err error) {
	if m.ssh != nil && m.ssh.runtime != nil {
		_ = m.ssh.runtime.Close()
	}
	if err != nil {
		m.lastErr = err
		label := "ssh"
		objectID := ""
		if m.ssh != nil {
			label = m.ssh.request.machineLabel
			objectID = m.selectedID
		}
		m.addEvent(domain.Event{Kind: "ssh-error", Label: label, ObjectID: objectID, Message: err.Error()})
	}
	m.ssh = nil
}

func (m *Model) resizeSSH() {
	if m.ssh == nil {
		return
	}
	box := sshPopupBox(m.width, m.canvasHeight())
	cols := max(1, box.width-4)
	rows := max(1, box.height-4)
	m.ssh.request.cols = cols
	m.ssh.request.rows = rows
	if m.ssh.term != nil {
		m.ssh.term.Resize(cols, rows)
	}
	if m.ssh.runtime != nil {
		_ = m.ssh.runtime.Resize(cols, rows)
	}
}

func sshArgs(model, machineID string) []string {
	return []string{"ssh", "-m", model, machineID}
}

func jujuMachineID(node domain.Node) (string, bool) {
	if id := strings.TrimSpace(node.Metadata["machine_id"]); id != "" {
		return id, true
	}
	parts := strings.SplitN(node.ID, ":", 3)
	if len(parts) == 3 && parts[0] == "machine" && strings.TrimSpace(parts[2]) != "" {
		return parts[2], true
	}
	return "", false
}

func normalSSHExit(err error) bool {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		return true
	}
	if errors.Is(err, syscall.EIO) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "input/output error")
}

func sshKeyBytes(msg tea.KeyMsg) []byte {
	key := tea.Key(msg)
	out := []byte{}
	if key.Alt {
		out = append(out, '\x1b')
	}
	switch key.Type {
	case tea.KeyRunes:
		return append(out, string(key.Runes)...)
	case tea.KeySpace:
		return append(out, ' ')
	case tea.KeyUp:
		return append(out, "\x1b[A"...)
	case tea.KeyDown:
		return append(out, "\x1b[B"...)
	case tea.KeyRight:
		return append(out, "\x1b[C"...)
	case tea.KeyLeft:
		return append(out, "\x1b[D"...)
	case tea.KeyHome:
		return append(out, "\x1b[H"...)
	case tea.KeyEnd:
		return append(out, "\x1b[F"...)
	case tea.KeyPgUp:
		return append(out, "\x1b[5~"...)
	case tea.KeyPgDown:
		return append(out, "\x1b[6~"...)
	case tea.KeyDelete:
		return append(out, "\x1b[3~"...)
	case tea.KeyInsert:
		return append(out, "\x1b[2~"...)
	default:
		if key.Type >= 0 && key.Type <= 31 || key.Type == 127 {
			return append(out, byte(key.Type))
		}
	}
	return out
}

func (m Model) renderSSHOverlay(body string, width, height int) string {
	if m.ssh == nil || width <= 0 || height <= 0 {
		return body
	}
	lines := normalizeANSIBlock(body, width, height)
	box := sshPopupBox(width, height)
	popup := strings.Split(m.renderSSHPopup(box.width, box.height), "\n")
	for row := 0; row < box.height && row < len(popup); row++ {
		y := box.y + row
		if y < 0 || y >= len(lines) {
			continue
		}
		left := strings.Repeat(" ", box.x)
		right := strings.Repeat(" ", max(0, width-box.x-box.width))
		lines[y] = left + popup[row] + right
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderSSHPopup(width, height int) string {
	canvas := NewCanvas(width, height)
	borderStyle := m.styles.Title
	canvas.BoxWithBorder(0, 0, width, height, borderStyle, RoundedBorderRunes)
	title := " SSH: " + m.ssh.request.machineLabel + " "
	canvas.Text(2, 0, truncate(title, max(0, width-4)), borderStyle)
	hint := "Ctrl+] close"
	command := truncate(m.ssh.request.commandLine, max(0, width-4-len(hint)-2))
	canvas.Text(2, 1, command, m.styles.Title)
	if width-len(hint)-2 > 2 {
		canvas.Text(width-len(hint)-2, 1, hint, m.styles.Dim)
	}
	if height > 3 {
		canvas.HLine(1, width-2, 2, '─', m.styles.Dim)
	}
	lines := m.sshTerminalLines(width-4, max(0, height-4))
	for index, line := range lines {
		y := 3 + index
		if y >= height-1 {
			break
		}
		canvas.Text(2, y, truncate(line, width-4), m.styles.Title)
	}
	return canvas.Render()
}

func (m Model) sshTerminalLines(width, height int) []string {
	lines := make([]string, height)
	if height <= 0 || width <= 0 {
		return lines
	}
	if m.ssh == nil || m.ssh.term == nil {
		return lines
	}
	if m.ssh.runtime == nil {
		lines[0] = "Opening SSH session..."
		if height > 1 {
			lines[1] = m.ssh.request.commandLine
		}
		return lines
	}
	content := strings.TrimSuffix(m.ssh.term.String(), "\n")
	terminalLines := strings.Split(content, "\n")
	for index := 0; index < height && index < len(terminalLines); index++ {
		lines[index] = fitCellLine(terminalLines[index], width)
	}
	cursor := m.ssh.term.Cursor()
	if m.ssh.term.CursorVisible() && cursor.Y >= 0 && cursor.Y < height && cursor.X >= 0 && cursor.X < width {
		runes := []rune(fitCellLine(lines[cursor.Y], width))
		for len(runes) < width {
			runes = append(runes, ' ')
		}
		if runes[cursor.X] == ' ' {
			runes[cursor.X] = '_'
		}
		lines[cursor.Y] = string(runes)
	}
	return lines
}

func sshPopupBox(width, height int) renderBox {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	boxWidth := max(20, width*80/100)
	boxHeight := max(8, height*80/100)
	boxWidth = min(width, boxWidth)
	boxHeight = min(height, boxHeight)
	return renderBox{
		x:      (width - boxWidth) / 2,
		y:      (height - boxHeight) / 2,
		width:  boxWidth,
		height: boxHeight,
	}
}

func normalizeANSIBlock(value string, width, height int) []string {
	raw := strings.Split(value, "\n")
	lines := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(raw) {
			lines[i] = ansi.Truncate(raw[i], width, "")
		}
		lines[i] += strings.Repeat(" ", max(0, width-ansi.StringWidth(lines[i])))
	}
	return lines
}

func fitCellLine(value string, width int) string {
	runes := []rune{}
	for _, ch := range value {
		chWidth := runewidth.RuneWidth(ch)
		if chWidth <= 0 {
			chWidth = 1
		}
		if len(runes)+chWidth > width {
			break
		}
		runes = append(runes, ch)
		for offset := 1; offset < chWidth; offset++ {
			runes = append(runes, ' ')
		}
	}
	return string(runes)
}
