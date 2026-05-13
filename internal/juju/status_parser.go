package juju

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/anvial/juju-watch/internal/domain"
)

type rawStatus struct {
	Model        rawModel                  `json:"model"`
	Applications map[string]rawApplication `json:"applications"`
	Machines     map[string]rawMachine     `json:"machines"`
	Relations    []rawRelation             `json:"relations"`
	Storage      json.RawMessage           `json:"storage"`
}

type rawModel struct {
	Name        string         `json:"name"`
	Controller  string         `json:"controller"`
	Cloud       string         `json:"cloud"`
	Region      string         `json:"region"`
	Version     string         `json:"version"`
	ModelStatus rawStatusValue `json:"model-status"`
	Status      rawStatusValue `json:"status"`
}

type rawApplication struct {
	Charm             string                              `json:"charm"`
	CharmChannel      string                              `json:"charm-channel"`
	CharmVersion      string                              `json:"charm-version"`
	ApplicationStatus rawStatusValue                      `json:"application-status"`
	Status            rawStatusValue                      `json:"status"`
	Units             map[string]rawUnit                  `json:"units"`
	Relations         map[string][]rawApplicationRelation `json:"relations"`
	EndpointBindings  map[string]string                   `json:"endpoint-bindings"`
	Exposed           bool                                `json:"exposed"`
}

type rawUnit struct {
	Machine        string             `json:"machine"`
	PublicAddress  string             `json:"public-address"`
	OpenedPorts    []string           `json:"opened-ports"`
	OpenPorts      []string           `json:"open-ports"`
	WorkloadStatus rawStatusValue     `json:"workload-status"`
	JujuStatus     rawStatusValue     `json:"juju-status"`
	AgentStatus    rawStatusValue     `json:"agent-status"`
	Leader         bool               `json:"leader"`
	Subordinates   map[string]rawUnit `json:"subordinates"`
}

type rawMachine struct {
	InstanceID    string         `json:"instance-id"`
	DNSName       string         `json:"dns-name"`
	IPAddresses   []string       `json:"ip-addresses"`
	IPAddress     string         `json:"ip-address"`
	JujuStatus    rawStatusValue `json:"juju-status"`
	MachineStatus rawStatusValue `json:"machine-status"`
	Status        rawStatusValue `json:"status"`
}

type rawRelation struct {
	Endpoints []rawEndpoint `json:"endpoints"`
}

type rawEndpoint struct {
	ApplicationName string `json:"application-name"`
	Name            string `json:"name"`
	Role            string `json:"role"`
	Interface       string `json:"interface"`
}

type rawApplicationRelation struct {
	RelatedApplication string `json:"related-application"`
	Interface          string `json:"interface"`
	Scope              string `json:"scope"`
}

func (r *rawApplicationRelation) UnmarshalJSON(data []byte) error {
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		r.RelatedApplication = asString
		return nil
	}
	type relation rawApplicationRelation
	var asObject relation
	if err := json.Unmarshal(data, &asObject); err != nil {
		return err
	}
	*r = rawApplicationRelation(asObject)
	return nil
}

type rawStorage struct {
	Kind        string                `json:"kind"`
	Status      rawStatusValue        `json:"status"`
	Attachments rawStorageAttachments `json:"attachments"`
}

type rawStorageAttachments struct {
	Units    map[string]rawStorageAttachment `json:"units"`
	Machines map[string]rawStorageAttachment `json:"machines"`
}

type rawStorageAttachment struct {
	Location string `json:"location"`
	Machine  string `json:"machine"`
}

type rawStatusValue struct {
	Current string `json:"current"`
	Message string `json:"message"`
	Since   string `json:"since"`
	Version string `json:"version"`
}

func ParseStatus(defaultModel string, data []byte) (domain.State, error) {
	data, err := extractJSON(data)
	if err != nil {
		return domain.State{}, err
	}
	var raw rawStatus
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.State{}, CommandError{Kind: ErrInvalidJSON, Output: preview(data), Err: err}
	}

	model := raw.Model.Name
	if model == "" {
		model = defaultModel
	}
	if model == "" {
		return domain.State{}, fmt.Errorf("missing model name in status output")
	}

	state := domain.NewState(model)
	state.Controller = raw.Model.Controller
	state.Cloud = raw.Model.Cloud
	state.Region = raw.Model.Region
	state.Version = raw.Model.Version
	state.Status = normalized(raw.Model.ModelStatus, raw.Model.Status)
	state.StatusMsg = firstNonEmpty(raw.Model.ModelStatus.Message, raw.Model.Status.Message)
	state.ObservedAt = time.Now()

	if raw.Applications == nil {
		raw.Applications = map[string]rawApplication{}
	}
	for _, appName := range sortedRawApplicationKeys(raw.Applications) {
		rawApp := raw.Applications[appName]
		app := domain.Application{
			Name:          appName,
			Charm:         rawApp.Charm,
			CharmChannel:  rawApp.CharmChannel,
			CharmVersion:  rawApp.CharmVersion,
			Status:        normalized(rawApp.ApplicationStatus, rawApp.Status),
			StatusMessage: firstNonEmpty(rawApp.ApplicationStatus.Message, rawApp.Status.Message),
			Metadata: map[string]string{
				"exposed": fmt.Sprintf("%t", rawApp.Exposed),
			},
		}
		for endpoint, targets := range rawApp.Relations {
			for _, target := range targets {
				app.Relations = append(app.Relations, domain.EndpointID(appName, endpoint)+"->"+target.RelatedApplication)
			}
		}
		sort.Strings(app.Relations)
		state.Applications[appName] = app

		for _, unitName := range sortedRawUnitKeys(rawApp.Units) {
			addUnit(state, appName, unitName, rawApp.Units[unitName])
		}
	}

	for _, machineID := range sortedRawMachineKeys(raw.Machines) {
		rawMachine := raw.Machines[machineID]
		statusValue := rawMachine.MachineStatus
		if statusValue.Current == "" {
			statusValue = rawMachine.Status
		}
		if statusValue.Current == "" {
			statusValue = rawMachine.JujuStatus
		}
		machine := domain.Machine{
			ID:            machineID,
			InstanceID:    rawMachine.InstanceID,
			DNSName:       rawMachine.DNSName,
			IPAddress:     firstNonEmpty(rawMachine.IPAddress, first(rawMachine.IPAddresses)),
			Status:        domain.NormalizeStatus(statusValue.Current),
			StatusMessage: statusValue.Message,
			Metadata:      map[string]string{},
		}
		state.Machines[machineID] = machine
	}

	for unitName, unit := range state.Units {
		if unit.MachineID == "" {
			continue
		}
		machine := state.Machines[unit.MachineID]
		if machine.ID == "" {
			machine = domain.Machine{
				ID:       unit.MachineID,
				Status:   domain.StatusUnknown,
				Metadata: map[string]string{},
			}
		}
		machine.Units = append(machine.Units, unitName)
		sort.Strings(machine.Units)
		state.Machines[unit.MachineID] = machine
	}

	state.Relations = parseRelations(raw)
	state.Storage = parseStorage(raw.Storage)
	return state, nil
}

func addUnit(state domain.State, appName, unitName string, rawUnit rawUnit) {
	unit := domain.Unit{
		Name:            unitName,
		AppName:         appName,
		MachineID:       rawUnit.Machine,
		PublicAddress:   rawUnit.PublicAddress,
		WorkloadStatus:  domain.NormalizeStatus(rawUnit.WorkloadStatus.Current),
		WorkloadMessage: rawUnit.WorkloadStatus.Message,
		AgentStatus:     normalized(rawUnit.JujuStatus, rawUnit.AgentStatus),
		AgentMessage:    firstNonEmpty(rawUnit.JujuStatus.Message, rawUnit.AgentStatus.Message),
		Ports:           unitPorts(rawUnit),
		Leader:          rawUnit.Leader,
		Metadata:        map[string]string{},
	}
	sort.Strings(unit.Ports)
	state.Units[unitName] = unit
	app := state.Applications[appName]
	app.Units = append(app.Units, unitName)
	sort.Strings(app.Units)
	state.Applications[appName] = app

	for subName, sub := range rawUnit.Subordinates {
		subApp := appNameFromUnit(subName)
		if _, ok := state.Applications[subApp]; !ok {
			state.Applications[subApp] = domain.Application{
				Name:     subApp,
				Status:   domain.NormalizeStatus(sub.WorkloadStatus.Current),
				Metadata: map[string]string{"subordinate": "true"},
			}
		}
		addUnit(state, subApp, subName, sub)
	}
}

func parseRelations(raw rawStatus) []domain.Relation {
	relations := []domain.Relation{}
	seen := map[string]struct{}{}
	for _, rawRel := range raw.Relations {
		if len(rawRel.Endpoints) < 2 {
			continue
		}
		a := rawRel.Endpoints[0]
		b := rawRel.Endpoints[1]
		rel := domain.Relation{
			EndpointA: domain.EndpointID(a.ApplicationName, a.Name),
			EndpointB: domain.EndpointID(b.ApplicationName, b.Name),
			AppA:      a.ApplicationName,
			AppB:      b.ApplicationName,
			RoleA:     a.Role,
			RoleB:     b.Role,
			Interface: firstNonEmpty(a.Interface, b.Interface),
		}
		id := domain.RelationID(raw.Model.Name, rel.EndpointA, rel.EndpointB)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		relations = append(relations, rel)
	}
	if len(relations) > 0 {
		sortRelations(relations)
		return relations
	}

	for appName, app := range raw.Applications {
		for endpoint, targets := range app.Relations {
			for _, target := range targets {
				if target.RelatedApplication == "" || target.RelatedApplication == appName {
					continue
				}
				otherEndpoint := reciprocalEndpoint(raw.Applications, appName, target.RelatedApplication, target.Interface)
				rel := domain.Relation{
					EndpointA: domain.EndpointID(appName, endpoint),
					EndpointB: domain.EndpointID(target.RelatedApplication, otherEndpoint),
					AppA:      appName,
					AppB:      target.RelatedApplication,
					Interface: target.Interface,
				}
				id := domain.RelationID(raw.Model.Name, rel.EndpointA, rel.EndpointB)
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				relations = append(relations, rel)
			}
		}
	}
	sortRelations(relations)
	return relations
}

func parseStorage(data json.RawMessage) map[string]domain.Storage {
	storage := map[string]domain.Storage{}
	if len(bytes.TrimSpace(data)) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return storage
	}
	var wrapped struct {
		Storage map[string]rawStorage `json:"storage"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Storage) > 0 {
		return storageFromMap(wrapped.Storage)
	}
	var direct map[string]rawStorage
	if err := json.Unmarshal(data, &direct); err == nil {
		return storageFromMap(direct)
	}
	return storage
}

func storageFromMap(raw map[string]rawStorage) map[string]domain.Storage {
	storage := map[string]domain.Storage{}
	for id, value := range raw {
		item := domain.Storage{
			ID:            id,
			Kind:          value.Kind,
			Status:        domain.NormalizeStatus(value.Status.Current),
			StatusMessage: value.Status.Message,
			Metadata:      map[string]string{},
		}
		for unit, attachment := range value.Attachments.Units {
			item.Unit = unit
			item.Location = attachment.Location
			item.MachineID = attachment.Machine
			break
		}
		if item.MachineID == "" {
			for machine, attachment := range value.Attachments.Machines {
				item.MachineID = machine
				item.Location = attachment.Location
				break
			}
		}
		storage[id] = item
	}
	return storage
}

func extractJSON(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, CommandError{Kind: ErrInvalidJSON, Output: "empty output"}
	}
	if trimmed[0] == '{' {
		return trimmed, nil
	}
	startObject := bytes.IndexByte(trimmed, '{')
	start := startObject
	if start == -1 {
		return nil, CommandError{Kind: ErrInvalidJSON, Output: preview(trimmed)}
	}
	return bytes.TrimSpace(trimmed[start:]), nil
}

func preview(data []byte) string {
	value := strings.TrimSpace(string(data))
	if len(value) > 240 {
		return value[:240] + "..."
	}
	return value
}

func unitPorts(raw rawUnit) []string {
	ports := append([]string{}, raw.OpenPorts...)
	ports = append(ports, raw.OpenedPorts...)
	sort.Strings(ports)
	return ports
}

func reciprocalEndpoint(apps map[string]rawApplication, sourceApp, targetApp, iface string) string {
	target := apps[targetApp]
	for endpoint, relations := range target.Relations {
		for _, relation := range relations {
			if relation.RelatedApplication != sourceApp {
				continue
			}
			if iface == "" || relation.Interface == iface {
				return endpoint
			}
		}
	}
	return ""
}

func normalized(primary rawStatusValue, fallback rawStatusValue) domain.Status {
	if primary.Current != "" {
		return domain.NormalizeStatus(primary.Current)
	}
	return domain.NormalizeStatus(fallback.Current)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func appNameFromUnit(unit string) string {
	if i := strings.Index(unit, "/"); i > 0 {
		return unit[:i]
	}
	return unit
}

func sortedRawApplicationKeys(values map[string]rawApplication) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedRawUnitKeys(values map[string]rawUnit) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedRawMachineKeys(values map[string]rawMachine) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortRelations(relations []domain.Relation) {
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].EndpointA == relations[j].EndpointA {
			return relations[i].EndpointB < relations[j].EndpointB
		}
		return relations[i].EndpointA < relations[j].EndpointA
	})
}
