package main

import "github.com/cpuguy83/dockerclient"

type Volume struct {
	docker.Volume
	Containers []string
	Names      []string
}

type volStore struct {
	s      map[string]*Volume
	refMap map[string]map[*docker.Container]struct{}
}

func (v *volStore) Add(volume *Volume) {
	v.s[volume.Id()] = volume
}

func (v *volStore) AddRef(volume *Volume, container *docker.Container) {
	id := volume.Id()
	if _, exists := v.refMap[id]; !exists {
		v.refMap[id] = make(map[*docker.Container]struct{})
	}

	v.refMap[id][container] = struct{}{}
}

func (v *volStore) Get(id string) *Volume {
	return v.s[id]
}

func (v *volStore) CanRemove(volume *Volume) bool {
	var id = volume.Id()
	if len(v.refMap[id]) != 0 {
		return false
	}

	return true
}

func (v *volStore) Find(id string) *Volume {
	var vol *Volume
	if vol = v.Get(id); vol != nil {
		return vol
	}

	if vol = v.FindByName(id); vol != nil {
		return vol
	}

	if vol = v.FindByTruncatedID(id); vol != nil {
		return vol
	}

	return nil
}

func (v *volStore) FindByName(name string) *Volume {
	for _, vol := range v.s {
		for _, n := range vol.Names {
			if n == name {
				return vol
			}
		}
	}

	return nil
}

func (v *volStore) FindByTruncatedID(id string) *Volume {
	for _, vol := range v.s {
		volId := vol.Id()
		if len(volId) > 12 {
			volId = volId[:12]
		}
		if id == volId {
			return vol
		}
	}

	return nil
}
