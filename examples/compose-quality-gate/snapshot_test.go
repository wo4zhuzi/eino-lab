package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/compose"
)

func TestTopologySnapshotIsStableAndDetached(t *testing.T) {
	gate := newTestGate(t, ruleInspector{}, DefaultGateConfig())
	snapshot := gate.Topology()

	if snapshot.Name != graphName {
		t.Fatalf("Topology() name = %q, want %q", snapshot.Name, graphName)
	}
	wantNodeKeys := []string{nodeApprove, nodeInspect, nodeManual, nodeRemediate, nodeValidate}
	gotNodeKeys := make([]string, 0, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		gotNodeKeys = append(gotNodeKeys, node.Key)
		if node.Component == "" || node.InputType == "" || node.OutputType == "" {
			t.Fatalf("Topology() node = %#v, want component and type metadata", node)
		}
	}
	if !reflect.DeepEqual(gotNodeKeys, wantNodeKeys) {
		t.Fatalf("Topology() node keys = %#v, want %#v", gotNodeKeys, wantNodeKeys)
	}

	wantTargets := []string{nodeApprove, nodeManual, nodeRemediate}
	if len(snapshot.Branches) != 1 || snapshot.Branches[0].From != nodeInspect ||
		!reflect.DeepEqual(snapshot.Branches[0].Targets, wantTargets) {
		t.Fatalf("Topology() branches = %#v", snapshot.Branches)
	}
	if len(snapshot.Edges) != 5 {
		t.Fatalf("Topology() edges = %#v, want 5 edges", snapshot.Edges)
	}

	snapshot.Nodes[0].Key = "mutated"
	snapshot.Branches[0].Targets[0] = "mutated"
	fresh := gate.Topology()
	if fresh.Nodes[0].Key == "mutated" || fresh.Branches[0].Targets[0] == "mutated" {
		t.Fatalf("Topology() returned mutable internal snapshot: %#v", fresh)
	}
}

func TestTopologySnapshotIncludesNestedGraphWithoutInstances(t *testing.T) {
	inner := compose.NewGraph[string, string]()
	identity := compose.InvokableLambda(func(_ context.Context, input string) (string, error) {
		return input, nil
	})
	if err := inner.AddLambdaNode("identity", identity, compose.WithNodeName("identity")); err != nil {
		t.Fatalf("AddLambdaNode() error = %v", err)
	}
	if err := inner.AddEdge(compose.START, "identity"); err != nil {
		t.Fatalf("AddEdge(START) error = %v", err)
	}
	if err := inner.AddEdge("identity", compose.END); err != nil {
		t.Fatalf("AddEdge(END) error = %v", err)
	}

	outer := compose.NewGraph[string, string]()
	if err := outer.AddGraphNode(
		"inner",
		inner,
		compose.WithGraphCompileOptions(compose.WithGraphName("inner_graph")),
	); err != nil {
		t.Fatalf("AddGraphNode() error = %v", err)
	}
	if err := outer.AddEdge(compose.START, "inner"); err != nil {
		t.Fatalf("AddEdge(START) error = %v", err)
	}
	if err := outer.AddEdge("inner", compose.END); err != nil {
		t.Fatalf("AddEdge(END) error = %v", err)
	}

	snapshotter := NewTopologySnapshotter()
	if _, err := outer.Compile(
		context.Background(),
		compose.WithGraphName("outer_graph"),
		compose.WithGraphCompileCallbacks(snapshotter),
	); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	snapshot := snapshotter.Snapshot()
	if len(snapshot.Nodes) != 1 || snapshot.Nodes[0].Nested == nil {
		t.Fatalf("Snapshot() nodes = %#v, want nested graph metadata", snapshot.Nodes)
	}
	nested := snapshot.Nodes[0].Nested
	if nested.Name != "inner_graph" || len(nested.Nodes) != 1 || nested.Nodes[0].Key != "identity" {
		t.Fatalf("Snapshot() nested graph = %#v", nested)
	}
}

func TestTopologySnapshotterSupportsConcurrentCompiles(t *testing.T) {
	snapshotter := NewTopologySnapshotter()
	const compiles = 16
	errorsCh := make(chan error, compiles)
	var wait sync.WaitGroup

	for i := 0; i < compiles; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			graph := compose.NewGraph[string, string]()
			identity := compose.InvokableLambda(func(_ context.Context, input string) (string, error) {
				return input, nil
			})
			if err := graph.AddLambdaNode("identity", identity); err != nil {
				errorsCh <- fmt.Errorf("compile %d add node: %w", index, err)
				return
			}
			if err := graph.AddEdge(compose.START, "identity"); err != nil {
				errorsCh <- fmt.Errorf("compile %d add start edge: %w", index, err)
				return
			}
			if err := graph.AddEdge("identity", compose.END); err != nil {
				errorsCh <- fmt.Errorf("compile %d add end edge: %w", index, err)
				return
			}
			_, err := graph.Compile(
				context.Background(),
				compose.WithGraphName(fmt.Sprintf("graph-%02d", index)),
				compose.WithGraphCompileCallbacks(snapshotter),
			)
			if err != nil {
				errorsCh <- fmt.Errorf("compile %d: %w", index, err)
			}
		}(i)
	}

	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Error(err)
	}

	snapshot := snapshotter.Snapshot()
	if !strings.HasPrefix(snapshot.Name, "graph-") || len(snapshot.Nodes) != 1 || snapshot.Nodes[0].Key != "identity" {
		t.Fatalf("Snapshot() after concurrent compiles = %#v", snapshot)
	}
}
