package layout

import (
	"context"
	"math"
	"sort"

	"github.com/anvial/juju-watch/internal/domain"
)

type Schema struct{}

const (
	leftMargin       = 4
	topMargin        = 2
	appBoxWidth      = 24
	appVerticalGap   = 3
	storageBoxWidth  = 28
	storageBoxHeight = 4
	machineBoxWidth  = 28

	machineAppGapMin      = 8
	appRelationGapMin     = 16
	appRelationLabelWidth = 14

	appColumnLeft  = 0
	appColumnRight = 1
)

type schemaColumns struct {
	machineX  int
	appLeftX  int
	appRightX int
}

func (Schema) Layout(_ context.Context, graph domain.Graph, previous *domain.Graph, opts Options) (domain.Graph, error) {
	unitsByApp := unitsByApplication(graph)
	storageByUnit := storageByUnit(graph)
	columns := calculateSchemaColumns(opts.Width)
	previousApps := previousApplicationLayout(previous, columns)

	appHeights := map[string]int{}
	for _, id := range appNodeIDs(graph) {
		appHeights[id] = appHeight(len(unitsByApp[id]), nestedStorageCount(unitsByApp[id], storageByUnit))
	}
	appColumns := applicationColumns(graph, previousApps, appHeights)
	appLeftIDs, appRightIDs := orderedApplicationColumns(graph, previousApps, appColumns)

	appLeftY := packColumn(appLeftIDs, appHeights, nil, topMargin)
	appRightY := packColumn(appRightIDs, appHeights, nil, topMargin)
	for _, id := range appLeftIDs {
		setTarget(&graph, id, float64(columns.appLeftX), float64(appLeftY[id]))
	}
	for _, id := range appRightIDs {
		setTarget(&graph, id, float64(columns.appRightX), float64(appRightY[id]))
	}

	for _, appID := range appNodeIDs(graph) {
		appNode := graph.Nodes[appID]
		unitIDs := unitsByApp[appID]
		sort.Strings(unitIDs)
		row := 3
		for _, unitID := range unitIDs {
			setTarget(&graph, unitID, appNode.Target.X+2, appNode.Target.Y+float64(row))
			row++
			storageIDs := storageByUnit[unitID]
			sort.Strings(storageIDs)
			for _, storageID := range storageIDs {
				setTarget(&graph, storageID, appNode.Target.X+4, appNode.Target.Y+float64(row))
				row++
			}
		}
	}

	machineIDs := typedNodeIDs(graph, domain.NodeMachine)
	machineHeights := map[string]int{}
	machineDesired := map[string]int{}
	for _, id := range machineIDs {
		machineHeights[id] = machineHeight(unitCountForMachine(graph, id))
		machineDesired[id] = machineDesiredTop(graph, id, appHeights)
	}
	sortByDesired(graph, machineIDs, machineDesired)
	machineY := packColumn(machineIDs, machineHeights, machineDesired, topMargin)
	for _, id := range machineIDs {
		setTarget(&graph, id, float64(columns.machineX), float64(machineY[id]))
	}

	layoutUnattachedStorage(&graph, columns, appLeftIDs, appRightIDs, appLeftY, appRightY, appHeights, storageByUnit)

	if previous != nil {
		for id, node := range graph.Nodes {
			if oldNode, ok := previous.Nodes[id]; ok {
				node.Current = oldNode.Current
				graph.Nodes[id] = node
			}
		}
	}
	return graph, nil
}

func calculateSchemaColumns(width int) schemaColumns {
	minGaps := []int{machineAppGapMin, appRelationGapMin}
	boxWidth := machineBoxWidth + appBoxWidth + appBoxWidth
	minContentWidth := boxWidth + sumInts(minGaps)
	available := width - leftMargin*2
	if available < minContentWidth {
		available = minContentWidth
	}

	extra := available - minContentWidth
	gaps := append([]int{}, minGaps...)
	placementExtra := extra / 3
	relationExtra := extra - placementExtra
	gaps[0] += placementExtra
	gaps[1] += relationExtra

	machineX := leftMargin
	appLeftX := machineX + machineBoxWidth + gaps[0]
	appRightX := appLeftX + appBoxWidth + gaps[1]
	return schemaColumns{
		machineX:  machineX,
		appLeftX:  appLeftX,
		appRightX: appRightX,
	}
}

type previousApplications struct {
	columns map[string]int
	ranks   map[string]int
	y       map[string]float64
}

func previousApplicationLayout(previous *domain.Graph, columns schemaColumns) previousApplications {
	placement := previousApplications{
		columns: map[string]int{},
		ranks:   map[string]int{},
		y:       map[string]float64{},
	}
	if previous == nil {
		return placement
	}

	xs := []float64{}
	for _, node := range previous.Nodes {
		if node.Type == domain.NodeApplication {
			xs = append(xs, node.Target.X)
		}
	}
	xs = uniqueSortedFloats(xs)

	boundary := 0.0
	useBoundary := len(xs) >= 2
	if useBoundary {
		boundary = (xs[0] + xs[len(xs)-1]) / 2
	}

	for id, node := range previous.Nodes {
		if node.Type != domain.NodeApplication {
			continue
		}
		column := appColumnLeft
		if useBoundary {
			if node.Target.X > boundary {
				column = appColumnRight
			}
		} else if math.Abs(node.Target.X-float64(columns.appRightX)) < math.Abs(node.Target.X-float64(columns.appLeftX)) {
			column = appColumnRight
		}
		placement.columns[id] = column
		placement.y[id] = node.Target.Y
	}

	for _, column := range []int{appColumnLeft, appColumnRight} {
		ids := []string{}
		for id, previousColumn := range placement.columns {
			if previousColumn == column {
				ids = append(ids, id)
			}
		}
		sort.SliceStable(ids, func(i, j int) bool {
			left := previous.Nodes[ids[i]]
			right := previous.Nodes[ids[j]]
			if left.Target.Y != right.Target.Y {
				return left.Target.Y < right.Target.Y
			}
			if left.Label != right.Label {
				return left.Label < right.Label
			}
			return left.ID < right.ID
		})
		for rank, id := range ids {
			placement.ranks[id] = rank
		}
	}

	return placement
}

func applicationColumns(graph domain.Graph, previous previousApplications, appHeights map[string]int) map[string]int {
	scores := relationDirectionScores(graph)
	columns := map[string]int{}
	leftHeight := 0
	rightHeight := 0

	for _, id := range appNodeIDs(graph) {
		column, ok := previous.columns[id]
		if !ok {
			continue
		}
		columns[id] = column
		if column == appColumnRight {
			rightHeight += appHeights[id] + appVerticalGap
		} else {
			leftHeight += appHeights[id] + appVerticalGap
		}
	}

	newIDs := []string{}
	for _, id := range appNodeIDs(graph) {
		if _, ok := columns[id]; !ok {
			newIDs = append(newIDs, id)
		}
	}
	sortApplicationColumn(graph, newIDs)

	for _, id := range newIDs {
		height := appHeights[id] + appVerticalGap
		column, ok := columnFromPinnedRelation(graph, id, previous.columns)
		if !ok {
			switch {
			case scores[id] > 0:
				column = appColumnLeft
			case scores[id] < 0:
				column = appColumnRight
			case leftHeight <= rightHeight:
				column = appColumnLeft
			default:
				column = appColumnRight
			}
		}
		columns[id] = column
		if column == appColumnRight {
			rightHeight += height
		} else {
			leftHeight += height
		}
	}

	return columns
}

func relationDirectionScores(graph domain.Graph) map[string]int {
	scores := map[string]int{}
	for _, id := range appNodeIDs(graph) {
		scores[id] = 0
	}
	for _, edge := range graph.Edges {
		if edge.Type != domain.EdgeRelation {
			continue
		}
		if graph.Nodes[edge.SourceID].Type != domain.NodeApplication || graph.Nodes[edge.TargetID].Type != domain.NodeApplication {
			continue
		}
		scores[edge.SourceID]++
		scores[edge.TargetID]--
	}
	return scores
}

func columnFromPinnedRelation(graph domain.Graph, id string, pinnedColumns map[string]int) (int, bool) {
	leftVotes := 0
	rightVotes := 0
	for _, edgeID := range sortedEdgeIDs(graph) {
		edge := graph.Edges[edgeID]
		if edge.Type != domain.EdgeRelation {
			continue
		}
		otherID := ""
		switch id {
		case edge.SourceID:
			otherID = edge.TargetID
		case edge.TargetID:
			otherID = edge.SourceID
		default:
			continue
		}
		otherColumn, ok := pinnedColumns[otherID]
		if !ok {
			continue
		}
		if otherColumn == appColumnRight {
			leftVotes++
		} else {
			rightVotes++
		}
	}
	if leftVotes == rightVotes {
		return 0, false
	}
	if leftVotes > rightVotes {
		return appColumnLeft, true
	}
	return appColumnRight, true
}

func orderedApplicationColumns(graph domain.Graph, previous previousApplications, columns map[string]int) ([]string, []string) {
	appLeftIDs := []string{}
	appRightIDs := []string{}
	for _, id := range appNodeIDs(graph) {
		if columns[id] == appColumnRight {
			appRightIDs = append(appRightIDs, id)
			continue
		}
		appLeftIDs = append(appLeftIDs, id)
	}
	if len(previous.columns) == 0 {
		sortApplicationColumn(graph, appLeftIDs)
		sortApplicationColumn(graph, appRightIDs)
		return appLeftIDs, appRightIDs
	}

	return orderApplicationColumn(graph, previous, appLeftIDs), orderApplicationColumn(graph, previous, appRightIDs)
}

func orderApplicationColumn(graph domain.Graph, previous previousApplications, ids []string) []string {
	existing := []string{}
	added := []string{}
	for _, id := range ids {
		if _, ok := previous.ranks[id]; ok {
			existing = append(existing, id)
		} else {
			added = append(added, id)
		}
	}
	sort.SliceStable(existing, func(i, j int) bool {
		return previous.ranks[existing[i]] < previous.ranks[existing[j]]
	})
	sortApplicationColumn(graph, added)

	ordered := append([]string{}, existing...)
	for _, id := range added {
		index, ok := insertionIndexForNewApplication(graph, previous, id, ordered)
		if !ok {
			ordered = append(ordered, id)
			continue
		}
		ordered = append(ordered, "")
		copy(ordered[index+1:], ordered[index:])
		ordered[index] = id
	}
	return ordered
}

func insertionIndexForNewApplication(graph domain.Graph, previous previousApplications, id string, ordered []string) (int, bool) {
	related := relationNeighborSet(graph, id)
	lastRelatedIndex := -1
	for index, orderedID := range ordered {
		if related[orderedID] {
			lastRelatedIndex = index
		}
	}
	if lastRelatedIndex >= 0 {
		return lastRelatedIndex + 1, true
	}

	relatedY := []float64{}
	for relatedID := range related {
		if y, ok := previous.y[relatedID]; ok {
			relatedY = append(relatedY, y)
		}
	}
	if len(relatedY) == 0 {
		return 0, false
	}
	sort.Float64s(relatedY)
	desiredY := relatedY[len(relatedY)/2]
	index := 0
	for index < len(ordered) {
		y, ok := previous.y[ordered[index]]
		if ok && y >= desiredY {
			break
		}
		index++
	}
	return index, true
}

func relationNeighborSet(graph domain.Graph, id string) map[string]bool {
	neighbors := map[string]bool{}
	for _, edge := range graph.Edges {
		if edge.Type != domain.EdgeRelation {
			continue
		}
		if edge.SourceID == id {
			neighbors[edge.TargetID] = true
		}
		if edge.TargetID == id {
			neighbors[edge.SourceID] = true
		}
	}
	return neighbors
}

func sortApplicationColumn(graph domain.Graph, ids []string) {
	sort.SliceStable(ids, func(i, j int) bool {
		left := graph.Nodes[ids[i]]
		right := graph.Nodes[ids[j]]
		if left.Label != right.Label {
			return left.Label < right.Label
		}
		return left.ID < right.ID
	})
}

func packColumn(ids []string, heights map[string]int, desired map[string]int, startY int) map[string]int {
	yByID := map[string]int{}
	cursor := startY
	for _, id := range ids {
		y := cursor
		if desiredY, ok := desired[id]; ok && desiredY > y {
			y = desiredY
		}
		yByID[id] = y
		cursor = y + heights[id] + appVerticalGap
	}
	return yByID
}

func sortByDesired(graph domain.Graph, ids []string, desired map[string]int) {
	sort.SliceStable(ids, func(i, j int) bool {
		leftDesired := desired[ids[i]]
		rightDesired := desired[ids[j]]
		if leftDesired != rightDesired {
			return leftDesired < rightDesired
		}
		left := graph.Nodes[ids[i]]
		right := graph.Nodes[ids[j]]
		if left.Status.Severity() != right.Status.Severity() {
			return left.Status.Severity() > right.Status.Severity()
		}
		if left.Label != right.Label {
			return left.Label < right.Label
		}
		return left.ID < right.ID
	})
}

func machineDesiredTop(graph domain.Graph, machineID string, appHeights map[string]int) int {
	centers := []int{}
	for _, edge := range graph.Edges {
		if edge.Type != domain.EdgeUnitOnMachine || edge.TargetID != machineID {
			continue
		}
		appID := appIDForUnit(graph, edge.SourceID)
		appNode, ok := graph.Nodes[appID]
		if !ok {
			continue
		}
		centers = append(centers, int(math.Round(appNode.Target.Y))+appHeights[appID]/2)
	}
	if len(centers) == 0 {
		return topMargin
	}
	return max(topMargin, medianInt(centers)-machineHeight(unitCountForMachine(graph, machineID))/2)
}

func layoutUnattachedStorage(graph *domain.Graph, columns schemaColumns, appLeftIDs, appRightIDs []string, appLeftY, appRightY, appHeights map[string]int, storageByUnit map[string][]string) {
	attached := attachedStorageSet(storageByUnit)
	unattached := []string{}
	for _, id := range typedNodeIDs(*graph, domain.NodeStorage) {
		if !attached[id] {
			unattached = append(unattached, id)
		}
	}
	if len(unattached) == 0 {
		return
	}

	leftBottom := columnBottom(appLeftIDs, appLeftY, appHeights)
	rightBottom := columnBottom(appRightIDs, appRightY, appHeights)
	x := columns.appLeftX
	y := leftBottom + appVerticalGap
	if rightBottom < leftBottom {
		x = columns.appRightX
		y = rightBottom + appVerticalGap
	}
	for _, id := range unattached {
		setTarget(graph, id, float64(x), float64(y))
		y += storageBoxHeight + appVerticalGap
	}
}

func columnBottom(ids []string, yByID, heights map[string]int) int {
	bottom := topMargin
	for _, id := range ids {
		bottom = max(bottom, yByID[id]+heights[id])
	}
	return bottom
}

func appHeight(unitCount int, storageCount ...int) int {
	rows := unitCount
	for _, count := range storageCount {
		rows += count
	}
	return max(5, 4+rows)
}

func machineHeight(unitCount int) int {
	return max(5, 5+unitCount)
}

func appIDForUnit(graph domain.Graph, unitID string) string {
	for _, edge := range graph.Edges {
		if edge.Type == domain.EdgeAppHasUnit && edge.TargetID == unitID {
			return edge.SourceID
		}
	}
	return ""
}

func unitCountForMachine(graph domain.Graph, machineID string) int {
	count := 0
	for _, edge := range graph.Edges {
		if edge.Type == domain.EdgeUnitOnMachine && edge.TargetID == machineID {
			count++
		}
	}
	return count
}

func boxesOverlap(a nodeBox, b nodeBox) bool {
	return a.X < b.X+b.Width &&
		a.X+a.Width > b.X &&
		a.Y < b.Y+b.Height &&
		a.Y+a.Height > b.Y
}

type nodeBox struct {
	ID     string
	X      int
	Y      int
	Width  int
	Height int
}

func graphBoxes(graph domain.Graph) []nodeBox {
	boxes := []nodeBox{}
	unitsByApp := unitsByApplication(graph)
	storageByUnit := storageByUnit(graph)
	attached := attachedStorageSet(storageByUnit)
	for id, node := range graph.Nodes {
		switch node.Type {
		case domain.NodeApplication:
			boxes = append(boxes, nodeBox{
				ID:     id,
				X:      int(math.Round(node.Target.X)),
				Y:      int(math.Round(node.Target.Y)),
				Width:  appBoxWidth,
				Height: appHeight(len(unitsByApp[id]), nestedStorageCount(unitsByApp[id], storageByUnit)),
			})
		case domain.NodeMachine:
			boxes = append(boxes, nodeBox{
				ID:     id,
				X:      int(math.Round(node.Target.X)),
				Y:      int(math.Round(node.Target.Y)),
				Width:  machineBoxWidth,
				Height: machineHeight(unitCountForMachine(graph, id)),
			})
		case domain.NodeStorage:
			if attached[id] {
				continue
			}
			boxes = append(boxes, nodeBox{
				ID:     id,
				X:      int(math.Round(node.Target.X)),
				Y:      int(math.Round(node.Target.Y)),
				Width:  storageBoxWidth,
				Height: storageBoxHeight,
			})
		}
	}
	return boxes
}

func setTarget(graph *domain.Graph, id string, x, y float64) {
	node := graph.Nodes[id]
	node.Target = domain.Position{X: x, Y: y}
	if node.Current == (domain.Position{}) {
		node.Current = node.Target
	}
	graph.Nodes[id] = node
}

func appNodeIDs(graph domain.Graph) []string {
	return typedNodeIDs(graph, domain.NodeApplication)
}

func typedNodeIDs(graph domain.Graph, nodeType domain.NodeType) []string {
	ids := []string{}
	for _, id := range graph.Order {
		node := graph.Nodes[id]
		if node.Type == nodeType {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func unitsByApplication(graph domain.Graph) map[string][]string {
	out := map[string][]string{}
	for _, edge := range graph.Edges {
		if edge.Type != domain.EdgeAppHasUnit {
			continue
		}
		out[edge.SourceID] = append(out[edge.SourceID], edge.TargetID)
	}
	return out
}

func storageByUnit(graph domain.Graph) map[string][]string {
	out := map[string][]string{}
	for _, edge := range graph.Edges {
		if edge.Type != domain.EdgeStorageAttached {
			continue
		}
		out[edge.SourceID] = append(out[edge.SourceID], edge.TargetID)
	}
	return out
}

func nestedStorageCount(unitIDs []string, storageByUnit map[string][]string) int {
	count := 0
	for _, unitID := range unitIDs {
		count += len(storageByUnit[unitID])
	}
	return count
}

func attachedStorageSet(storageByUnit map[string][]string) map[string]bool {
	out := map[string]bool{}
	for _, storageIDs := range storageByUnit {
		for _, storageID := range storageIDs {
			out[storageID] = true
		}
	}
	return out
}

func medianInt(values []int) int {
	sort.Ints(values)
	return values[len(values)/2]
}

func sumInts(values []int) int {
	sum := 0
	for _, value := range values {
		sum += value
	}
	return sum
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sortedEdgeIDs(graph domain.Graph) []string {
	ids := append([]string{}, graph.EdgeOrder...)
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	for id := range graph.Edges {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func uniqueSortedFloats(values []float64) []float64 {
	sort.Float64s(values)
	out := []float64{}
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
