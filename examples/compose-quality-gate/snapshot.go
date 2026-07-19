package main

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/cloudwego/eino/compose"
)

type TopologySnapshot struct {
	Name       string
	InputType  string
	OutputType string
	Nodes      []TopologyNode
	Edges      []TopologyEdge
	Branches   []TopologyBranch
}

type TopologyNode struct {
	Key        string
	Name       string
	Component  string
	InputType  string
	OutputType string
	Nested     *TopologySnapshot
}

type TopologyEdge struct {
	From string
	To   string
}

type TopologyBranch struct {
	From    string
	Targets []string
}

type TopologySnapshotter struct {
	mu       sync.RWMutex
	snapshot TopologySnapshot
}

func NewTopologySnapshotter() *TopologySnapshotter {
	return &TopologySnapshotter{}
}

func (s *TopologySnapshotter) OnFinish(_ context.Context, info *compose.GraphInfo) {
	snapshot := makeTopologySnapshot(info)
	s.mu.Lock()
	s.snapshot = snapshot
	s.mu.Unlock()
}

func (s *TopologySnapshotter) Snapshot() TopologySnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneTopologySnapshot(s.snapshot)
}

func makeTopologySnapshot(info *compose.GraphInfo) TopologySnapshot {
	if info == nil {
		return TopologySnapshot{}
	}

	snapshot := TopologySnapshot{
		Name:       info.Name,
		InputType:  reflectTypeName(info.InputType),
		OutputType: reflectTypeName(info.OutputType),
		Nodes:      make([]TopologyNode, 0, len(info.Nodes)),
	}
	for key, info := range info.Nodes {
		node := TopologyNode{
			Key:        key,
			Name:       info.Name,
			Component:  string(info.Component),
			InputType:  reflectTypeName(info.InputType),
			OutputType: reflectTypeName(info.OutputType),
		}
		if info.GraphInfo != nil {
			nested := makeTopologySnapshot(info.GraphInfo)
			node.Nested = &nested
		}
		snapshot.Nodes = append(snapshot.Nodes, node)
	}
	sort.Slice(snapshot.Nodes, func(i, j int) bool {
		return snapshot.Nodes[i].Key < snapshot.Nodes[j].Key
	})

	for from, targets := range info.Edges {
		for _, target := range targets {
			snapshot.Edges = append(snapshot.Edges, TopologyEdge{From: from, To: target})
		}
	}
	sort.Slice(snapshot.Edges, func(i, j int) bool {
		if snapshot.Edges[i].From == snapshot.Edges[j].From {
			return snapshot.Edges[i].To < snapshot.Edges[j].To
		}
		return snapshot.Edges[i].From < snapshot.Edges[j].From
	})

	for from, branches := range info.Branches {
		for _, branch := range branches {
			targets := make([]string, 0, len(branch.GetEndNode()))
			for target := range branch.GetEndNode() {
				targets = append(targets, target)
			}
			sort.Strings(targets)
			snapshot.Branches = append(snapshot.Branches, TopologyBranch{From: from, Targets: targets})
		}
	}
	sort.Slice(snapshot.Branches, func(i, j int) bool {
		if snapshot.Branches[i].From == snapshot.Branches[j].From {
			return strings.Join(snapshot.Branches[i].Targets, "\x00") < strings.Join(snapshot.Branches[j].Targets, "\x00")
		}
		return snapshot.Branches[i].From < snapshot.Branches[j].From
	})

	return snapshot
}

func cloneTopologySnapshot(source TopologySnapshot) TopologySnapshot {
	clone := source
	clone.Nodes = make([]TopologyNode, len(source.Nodes))
	for i, node := range source.Nodes {
		clone.Nodes[i] = node
		if node.Nested != nil {
			nested := cloneTopologySnapshot(*node.Nested)
			clone.Nodes[i].Nested = &nested
		}
	}
	clone.Edges = append([]TopologyEdge(nil), source.Edges...)
	clone.Branches = make([]TopologyBranch, len(source.Branches))
	for i, branch := range source.Branches {
		clone.Branches[i] = branch
		clone.Branches[i].Targets = append([]string(nil), branch.Targets...)
	}
	return clone
}

func reflectTypeName(value reflect.Type) string {
	if value == nil {
		return ""
	}
	return value.String()
}
