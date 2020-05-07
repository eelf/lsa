package main

import (
	"eelf.ru/lsa"
)

type Repository map[string]map[string]*lsa.Stat

func NewRepository() *Repository {
	r := make(Repository)
	return &r
}

func (r *Repository) AddDirIfNew(dir string) {
	if _, ok := (*r)[dir]; ok {
		return
	}
	(*r)[dir] = make(map[string]*lsa.Stat)
}

func (r *Repository) AddFileToDir(dir, file string, stat *lsa.Stat) {
	(*r)[dir][file] = stat
}

func (r *Repository) GetDirStat(dir string) map[string]*lsa.Stat {
	stat, ok := (*r)[dir]
	if !ok {
		return nil
	}

	return stat
}

func (r *Repository) SetDirStat(dir string, stat map[string]*lsa.Stat) {
	(*r)[dir] = stat
}

func (r *Repository) DelFile(dir, file string) {
	delete((*r)[dir], file)
}
