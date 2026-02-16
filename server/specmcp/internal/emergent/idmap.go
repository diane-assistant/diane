package emergent

import (
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// NodeIndex builds a dual-indexed map from ExpandNode slices.
// Entries are keyed by both ID and CanonicalID so that edge
// references (which may use either) always resolve.
type NodeIndex map[string]*graph.ExpandNode

// NewNodeIndex creates a NodeIndex from a slice of ExpandNodes.
func NewNodeIndex(nodes []*graph.ExpandNode) NodeIndex {
	idx := make(NodeIndex, len(nodes)*2)
	for _, n := range nodes {
		idx[n.ID] = n
		if n.CanonicalID != "" && n.CanonicalID != n.ID {
			idx[n.CanonicalID] = n
		}
	}
	return idx
}

// ObjectIndex builds a dual-indexed map from GraphObject slices.
// Entries are keyed by both ID and CanonicalID so that relationship
// SrcID/DstID references (which may use either) always resolve.
type ObjectIndex map[string]*graph.GraphObject

// NewObjectIndex creates an ObjectIndex from a slice of GraphObjects.
func NewObjectIndex(objs []*graph.GraphObject) ObjectIndex {
	idx := make(ObjectIndex, len(objs)*2)
	for _, o := range objs {
		idx[o.ID] = o
		if o.CanonicalID != "" && o.CanonicalID != o.ID {
			idx[o.CanonicalID] = o
		}
	}
	return idx
}

// IDSet builds a set that matches both the version-specific ID and
// canonical ID of an entity, so that edge endpoint comparisons work
// regardless of which ID variant the edge stores.
type IDSet map[string]bool

// NewIDSet creates an IDSet for a single entity identified by both IDs.
// Either id may be empty (in which case only the non-empty one is added).
func NewIDSet(id, canonicalID string) IDSet {
	s := make(IDSet, 2)
	if id != "" {
		s[id] = true
	}
	if canonicalID != "" && canonicalID != id {
		s[canonicalID] = true
	}
	return s
}

// TaskIndex builds a dual-indexed map from Task slices.
// The Task struct embeds an ID field but has no CanonicalID;
// callers that get tasks from ListTasks (which goes through
// ListRelationships→GetObjects) should use ObjectIndex instead
// for the underlying objects and only use this for quick status lookups.
//
// However, since tasks are fetched via GetObjects (which returns
// GraphObjects with CanonicalID), we pass the underlying objects
// alongside tasks to enable dual-indexing.
type TaskIndex struct {
	byID map[string]*Task
}

// NewTaskIndex creates a TaskIndex from tasks and their corresponding objects.
// If objects is nil, only task.ID is indexed (no canonical ID fallback).
func NewTaskIndex(tasks []*Task, objects []*graph.GraphObject) *TaskIndex {
	idx := &TaskIndex{
		byID: make(map[string]*Task, len(tasks)*2),
	}
	for _, t := range tasks {
		idx.byID[t.ID] = t
	}
	// Cross-index using objects' canonical IDs if available
	if len(objects) > 0 {
		objByID := make(map[string]*graph.GraphObject, len(objects))
		for _, o := range objects {
			objByID[o.ID] = o
		}
		for _, t := range tasks {
			if obj, ok := objByID[t.ID]; ok {
				if obj.CanonicalID != "" && obj.CanonicalID != t.ID {
					idx.byID[obj.CanonicalID] = t
				}
			}
		}
	}
	return idx
}

// Get looks up a task by any ID (version or canonical).
func (ti *TaskIndex) Get(id string) (*Task, bool) {
	t, ok := ti.byID[id]
	return t, ok
}

// Status returns the status of a task by any ID. Returns ("", false) if not found.
func (ti *TaskIndex) Status(id string) (string, bool) {
	t, ok := ti.byID[id]
	if !ok {
		return "", false
	}
	return t.Status, true
}

// CanonicalizeEdgeIDs normalizes edge SrcID/DstID references to use
// consistent IDs by resolving through a NodeIndex. If an edge endpoint
// is found in the index, its ID is replaced with the node's primary ID.
// This ensures all downstream map lookups use the same key.
func CanonicalizeEdgeIDs(edges []*graph.ExpandEdge, idx NodeIndex) {
	for _, e := range edges {
		if n, ok := idx[e.SrcID]; ok {
			e.SrcID = n.ID
		}
		if n, ok := idx[e.DstID]; ok {
			e.DstID = n.ID
		}
	}
}
