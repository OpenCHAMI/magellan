package cache

import "github.com/google/uuid"

type Storage[T any] interface {
	Save(id uuid.UUID, val T, varargs ...T) error
	Get(id uuid.UUID) (T, error)
	Update(id uuid.UUID, val T) error
	Delete(id uuid.UUID) error
}

type Compute struct{}
type BMC struct{}

type Node[T any] struct {
}

type NodeStorage struct {
	Storage[Node[Compute]]
}

type BMCStorage struct {
	Storage[Node[BMC]]
}

func (ns *NodeStorage) Save(id uuid.UUID, val Node[Compute], varargs ...Node[Compute]) {

}
