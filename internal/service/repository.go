package service

import (
	"errors"

	"k8s.io/apimachinery/pkg/types"
)

type ConfigMapEntity struct {
	Id       types.NamespacedName
	Pods     []types.NamespacedName
	DataHash string
}

type ConfigMapRepository interface {
	FindById(id types.NamespacedName) (ConfigMapEntity, error)
	FindByPod(id types.NamespacedName) ([]ConfigMapEntity, error)
	Save(entity ConfigMapEntity) (ConfigMapEntity, error)
	Delete(id types.NamespacedName) error
}

type InMemoryConfigMapRepository struct {
	entities map[types.NamespacedName]ConfigMapEntity
}

var Repository ConfigMapRepository = InMemoryConfigMapRepository{
	entities: make(map[types.NamespacedName]ConfigMapEntity),
}

func (r InMemoryConfigMapRepository) FindById(id types.NamespacedName) (ConfigMapEntity, error) {
	entity, ok := r.entities[id]
	if !ok {
		return ConfigMapEntity{}, errors.New("ConfigMap not found")
	}
	return entity, nil
}

func (r InMemoryConfigMapRepository) FindByPod(id types.NamespacedName) ([]ConfigMapEntity, error) {
	var entities []ConfigMapEntity
	for _, entity := range r.entities {
		for _, pod := range entity.Pods {
			if pod == id {
				entities = append(entities, entity)
			}
		}
	}
	return entities, nil
}

func (r InMemoryConfigMapRepository) Save(entity ConfigMapEntity) (ConfigMapEntity, error) {
	r.entities[entity.Id] = entity
	return entity, nil
}

func (r InMemoryConfigMapRepository) Delete(id types.NamespacedName) error {
	delete(r.entities, id)
	return nil
}
