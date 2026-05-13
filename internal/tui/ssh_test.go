package tui

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hinshun/vt10x"
)

func TestEnterOnSelectedMachineStartsSSHPopup(t *testing.T) {
	model := sshTestModel()
	runtime := &fakeSSHRuntime{}
	starter := &fakeSSHStarter{runtime: runtime}
	model.sshRun = starter
	model.selectedID = domain.MachineID("prod", "0")

	updated, cmd := model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected SSH start command")
	}
	if model.ssh == nil {
		t.Fatal("expected SSH popup state")
	}
	if model.ssh.request.commandLine != "juju ssh -m prod 0" {
		t.Fatalf("command line = %q", model.ssh.request.commandLine)
	}

	msg := cmd()
	started, ok := msg.(sshStartedMsg)
	if !ok {
		t.Fatalf("start command msg = %T, want sshStartedMsg", msg)
	}
	if started.err != nil {
		t.Fatalf("start error = %v", started.err)
	}
	if starter.starts != 1 {
		t.Fatalf("starts = %d, want 1", starter.starts)
	}
	if starter.request.model != "prod" || starter.request.machineID != "0" {
		t.Fatalf("request = %+v, want model prod machine 0", starter.request)
	}
}

func TestEnterOnNonMachineDoesNotStartSSH(t *testing.T) {
	model := sshTestModel()
	starter := &fakeSSHStarter{runtime: &fakeSSHRuntime{}}
	model.sshRun = starter
	model.selectedID = domain.AppID("prod", "postgresql")

	updated, cmd := model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("expected no SSH command for non-machine selection")
	}
	if model.ssh != nil {
		t.Fatal("expected no SSH popup for non-machine selection")
	}
	if starter.starts != 0 {
		t.Fatalf("starts = %d, want 0", starter.starts)
	}
}

func TestSearchEnterDoesNotStartSSH(t *testing.T) {
	model := sshTestModel()
	starter := &fakeSSHStarter{runtime: &fakeSSHRuntime{}}
	model.sshRun = starter
	model.selectedID = domain.MachineID("prod", "0")
	model.searching = true
	model.search.SetValue("postgresql")

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.ssh != nil {
		t.Fatal("search enter should not open SSH popup")
	}
	if starter.starts != 0 {
		t.Fatalf("starts = %d, want 0", starter.starts)
	}
}

func TestSSHCloseKeyClosesPopupAndOtherKeysForward(t *testing.T) {
	model := sshTestModel()
	runtime := &fakeSSHRuntime{}
	model.ssh = &sshPopup{
		request: sshRequest{model: "prod", machineID: "0", machineLabel: "machine 0", commandLine: "juju ssh -m prod 0", cols: 40, rows: 10},
		runtime: runtime,
		term:    vt10x.New(vt10x.WithSize(40, 10)),
		status:  "connected",
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ls")})
	model = updated.(Model)
	if got := string(runtime.writes); got != "ls" {
		t.Fatalf("forwarded bytes = %q, want ls", got)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlCloseBracket})
	model = updated.(Model)
	if model.ssh != nil {
		t.Fatal("close key should clear SSH popup")
	}
	if !runtime.closed {
		t.Fatal("close key should close runtime")
	}
}

func TestSSHOutputUpdatesTerminalAndEOFClosesPopup(t *testing.T) {
	model := sshTestModel()
	runtime := &fakeSSHRuntime{}
	req := sshRequest{model: "prod", machineID: "0", machineLabel: "machine 0", commandLine: "juju ssh -m prod 0", cols: 40, rows: 10}
	model.ssh = &sshPopup{request: req, term: vt10x.New(vt10x.WithSize(40, 10)), status: "connecting"}

	updated, _ := model.Update(sshStartedMsg{request: req, runtime: runtime})
	model = updated.(Model)
	if model.ssh == nil || model.ssh.runtime != runtime {
		t.Fatal("started SSH should attach runtime")
	}

	updated, _ = model.Update(sshOutputMsg{runtime: runtime, data: []byte("ubuntu@machine-0:~$ ")})
	model = updated.(Model)
	if !strings.Contains(model.ssh.term.String(), "ubuntu@machine-0") {
		t.Fatalf("terminal did not receive output: %q", model.ssh.term.String())
	}

	updated, _ = model.Update(sshExitMsg{runtime: runtime, err: io.EOF})
	model = updated.(Model)
	if model.ssh != nil {
		t.Fatal("EOF should close SSH popup")
	}
	if !runtime.closed {
		t.Fatal("EOF should close runtime")
	}
}

func TestSSHWindowResizeUpdatesTerminalAndRuntime(t *testing.T) {
	model := sshTestModel()
	runtime := &fakeSSHRuntime{}
	model.ssh = &sshPopup{
		request: sshRequest{model: "prod", machineID: "0", machineLabel: "machine 0", commandLine: "juju ssh -m prod 0", cols: 40, rows: 10},
		runtime: runtime,
		term:    vt10x.New(vt10x.WithSize(40, 10)),
		status:  "connected",
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)
	if model.ssh == nil {
		t.Fatal("resize should keep SSH popup open")
	}
	if model.ssh.request.cols != 92 || model.ssh.request.rows != 26 {
		t.Fatalf("ssh terminal size = %dx%d, want 92x26", model.ssh.request.cols, model.ssh.request.rows)
	}
	if len(runtime.resized) != 1 || runtime.resized[0].width != 92 || runtime.resized[0].height != 26 {
		t.Fatalf("runtime resized = %+v, want one 92x26 resize", runtime.resized)
	}
}

func TestSSHStartErrorSurfacesAsLastError(t *testing.T) {
	model := sshTestModel()
	req := sshRequest{model: "prod", machineID: "0", machineLabel: "machine 0", commandLine: "juju ssh -m prod 0", cols: 40, rows: 10}
	model.ssh = &sshPopup{request: req, term: vt10x.New(vt10x.WithSize(40, 10)), status: "connecting"}
	startErr := errors.New("juju missing")

	updated, _ := model.Update(sshStartedMsg{request: req, err: startErr})
	model = updated.(Model)
	if model.ssh != nil {
		t.Fatal("start error should close popup")
	}
	if !errors.Is(model.lastErr, startErr) {
		t.Fatalf("lastErr = %v, want %v", model.lastErr, startErr)
	}
}

func TestJujuMachineIDUsesMetadataThenGraphID(t *testing.T) {
	node := domain.Node{
		ID:       domain.MachineID("prod", "0"),
		Metadata: map[string]string{"machine_id": "2/lxd/0"},
	}
	if got, ok := jujuMachineID(node); !ok || got != "2/lxd/0" {
		t.Fatalf("metadata machine id = %q ok=%t, want 2/lxd/0 true", got, ok)
	}

	node.Metadata = nil
	if got, ok := jujuMachineID(node); !ok || got != "0" {
		t.Fatalf("fallback machine id = %q ok=%t, want 0 true", got, ok)
	}
}

func TestSSHArgsAndKeyEncoding(t *testing.T) {
	if got := strings.Join(sshArgs("prod", "0"), " "); got != "ssh -m prod 0" {
		t.Fatalf("ssh args = %q", got)
	}
	if got := string(sshKeyBytes(tea.KeyMsg{Type: tea.KeyEnter})); got != "\r" {
		t.Fatalf("enter bytes = %q, want carriage return", got)
	}
	if got := string(sshKeyBytes(tea.KeyMsg{Type: tea.KeyUp})); got != "\x1b[A" {
		t.Fatalf("up bytes = %q, want CSI up", got)
	}
}

func TestSSHPopupGeometryAndRendering(t *testing.T) {
	box := sshPopupBox(100, 30)
	if box.width != 80 || box.height != 24 || box.x != 10 || box.y != 3 {
		t.Fatalf("popup box = %+v, want centered 80x24 at 10,3", box)
	}

	model := sshTestModel()
	model.ssh = &sshPopup{
		request: sshRequest{model: "prod", machineID: "0", machineLabel: "machine 0", commandLine: "juju ssh -m prod 0", cols: 76, rows: 20},
		runtime: &fakeSSHRuntime{},
		term:    vt10x.New(vt10x.WithSize(76, 20)),
		status:  "connected",
	}
	_, _ = model.ssh.term.Write([]byte("ubuntu@machine-0:~$ "))

	body := strings.Repeat(strings.Repeat(".", 100)+"\n", 30)
	rendered := model.renderSSHOverlay(body, 100, 30)
	for _, want := range []string{"SSH: machine 0", "juju ssh -m prod 0", "Ctrl+] close", "ubuntu@machine-0"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("popup render missing %q: %q", want, rendered)
		}
	}
}

func sshTestModel() Model {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	model.cfg.Model = "prod"
	model.width = 100
	model.height = 30
	model.view = ViewTopology
	model.hasGraph = true
	model.graph = domain.NewGraph("prod")
	appID := domain.AppID("prod", "postgresql")
	machineID := domain.MachineID("prod", "0")
	model.graph.Nodes[appID] = domain.Node{ID: appID, Label: "postgresql", Type: domain.NodeApplication, Status: domain.StatusActive}
	model.graph.Nodes[machineID] = domain.Node{
		ID:       machineID,
		Label:    "machine 0",
		Type:     domain.NodeMachine,
		Status:   domain.StatusActive,
		Metadata: map[string]string{"machine_id": "0"},
	}
	model.graph.Order = []string{appID, machineID}
	return model
}

type fakeSSHStarter struct {
	runtime sshRuntime
	err     error
	request sshRequest
	starts  int
}

func (s *fakeSSHStarter) Start(req sshRequest) (sshRuntime, error) {
	s.starts++
	s.request = req
	return s.runtime, s.err
}

type fakeSSHRuntime struct {
	writes  []byte
	closed  bool
	resized []renderBox
}

func (r *fakeSSHRuntime) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (r *fakeSSHRuntime) Write(p []byte) (int, error) {
	r.writes = append(r.writes, p...)
	return len(p), nil
}

func (r *fakeSSHRuntime) Close() error {
	r.closed = true
	return nil
}

func (r *fakeSSHRuntime) Resize(cols, rows int) error {
	r.resized = append(r.resized, renderBox{width: cols, height: rows})
	return nil
}
