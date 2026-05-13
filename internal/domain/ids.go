package domain

import (
	"sort"
	"strings"
)

func ModelID(model string) string {
	return "model:" + cleanPart(model)
}

func AppID(model, app string) string {
	return "app:" + cleanPart(model) + ":" + cleanPart(app)
}

func UnitID(model, unit string) string {
	return "unit:" + cleanPart(model) + ":" + cleanPart(unit)
}

func MachineID(model, machine string) string {
	return "machine:" + cleanPart(model) + ":" + cleanPart(machine)
}

func StorageID(model, storage string) string {
	return "storage:" + cleanPart(model) + ":" + cleanPart(storage)
}

func EndpointID(app, endpoint string) string {
	if endpoint == "" {
		return cleanPart(app)
	}
	return cleanPart(app) + ":" + cleanPart(endpoint)
}

func RelationID(model, endpointA, endpointB string) string {
	parts := []string{cleanPart(endpointA), cleanPart(endpointB)}
	sort.Strings(parts)
	return "relation:" + cleanPart(model) + ":" + parts[0] + ":" + parts[1]
}

func EdgeID(edgeType EdgeType, sourceID, targetID string) string {
	parts := []string{sourceID, targetID}
	if edgeType == EdgeRelation {
		sort.Strings(parts)
	}
	return string(edgeType) + ":" + parts[0] + ":" + parts[1]
}

func cleanPart(value string) string {
	return strings.TrimSpace(value)
}
