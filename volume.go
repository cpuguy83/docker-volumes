package main

import "github.com/cpuguy83/dockerclient"

type Volume struct {
	docker.Volume
	ID         string
	Containers []string
	Names      []string
}

type volStore struct {
	s map[string]*Volume
}

func (v *volStore) Add(volume *Volume) {
	v.s[volume.ID] = volume
}

func (v *volStore) Get(id string) *Volume {
	return v.s[id]
}

func (v *volStore) CanRemove(volume *Volume) bool {
	if len(volume.Containers) != 0 {
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
		volId := vol.ID
		if len(volId) > 12 {
			volId = volId[:12]
		}
		if id == volId {
			return vol
		}
	}

	return nil
}
