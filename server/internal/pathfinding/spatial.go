// Package pathfinding provides A* pathfinding and spatial indexing for the game world.
package pathfinding

import (
	"sync"

	"github.com/google/uuid"
)

// CellSize is the side length (in tiles) of one AOI grid cell.
// A 128×128 map with CellSize=16 produces an 8×8 cell grid (64 cells total).
const CellSize = 16

type cellKey struct{ CX, CY int }

// SpatialGrid is a thread-safe nine-grid AOI (Area Of Interest) index.
// Roles are bucketed into axis-aligned square cells so that "nearby roles"
// queries touch at most 9 cells instead of scanning every role.
type SpatialGrid struct {
	mu    sync.RWMutex
	cells map[cellKey]map[uuid.UUID]struct{}
}

// NewSpatialGrid returns an empty SpatialGrid.
func NewSpatialGrid() *SpatialGrid {
	return &SpatialGrid{cells: make(map[cellKey]map[uuid.UUID]struct{})}
}

func cellOf(tileX, tileY int) cellKey {
	return cellKey{tileX / CellSize, tileY / CellSize}
}

// Insert adds id to the cell that contains (tileX, tileY).
func (g *SpatialGrid) Insert(id uuid.UUID, tileX, tileY int) {
	k := cellOf(tileX, tileY)
	g.mu.Lock()
	if g.cells[k] == nil {
		g.cells[k] = make(map[uuid.UUID]struct{})
	}
	g.cells[k][id] = struct{}{}
	g.mu.Unlock()
}

// Remove removes id from the cell that contains (tileX, tileY).
func (g *SpatialGrid) Remove(id uuid.UUID, tileX, tileY int) {
	k := cellOf(tileX, tileY)
	g.mu.Lock()
	delete(g.cells[k], id)
	g.mu.Unlock()
}

// Move relocates id from (oldX, oldY) to (newX, newY).
// If both coordinates fall in the same cell the call is a no-op.
func (g *SpatialGrid) Move(id uuid.UUID, oldX, oldY, newX, newY int) {
	oldK, newK := cellOf(oldX, oldY), cellOf(newX, newY)
	if oldK == newK {
		return
	}
	g.mu.Lock()
	delete(g.cells[oldK], id)
	if g.cells[newK] == nil {
		g.cells[newK] = make(map[uuid.UUID]struct{})
	}
	g.cells[newK][id] = struct{}{}
	g.mu.Unlock()
}

// NineGrid returns the IDs of all roles that occupy the 3×3 block of cells
// centred on the cell containing (tileX, tileY).
func (g *SpatialGrid) NineGrid(tileX, tileY int) []uuid.UUID {
	cx, cy := tileX/CellSize, tileY/CellSize
	g.mu.RLock()
	defer g.mu.RUnlock()

	var out []uuid.UUID
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			for id := range g.cells[cellKey{cx + dx, cy + dy}] {
				out = append(out, id)
			}
		}
	}
	return out
}
