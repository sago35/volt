package lockjson

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/vim-volt/go-volt/pathutil"
)

type repos []Repos
type profiles []Profile

type LockJSON struct {
	Version       int64    `json:"version"`
	TrxID         int64    `json:"trx_id"`
	ActiveProfile string   `json:"active_profile"`
	LoadVimrc     bool     `json:"load_vimrc"`
	LoadGvimrc    bool     `json:"load_gvimrc"`
	Repos         repos    `json:"repos"`
	Profiles      profiles `json:"profiles"`
}

type ReposType string

const (
	ReposGitType    ReposType = "git"
	ReposStaticType ReposType = "static"
)

type Repos struct {
	Type    ReposType `json:"type"`
	TrxID   int64     `json:"trx_id"`
	Path    string    `json:"path"`
	Version string    `json:"version"`
}

type profReposPath []string

type Profile struct {
	Name       string        `json:"name"`
	ReposPath  profReposPath `json:"repos_path"`
	LoadVimrc  bool          `json:"load_vimrc"`
	LoadGvimrc bool          `json:"load_gvimrc"`
}

func InitialLockJSON() *LockJSON {
	return &LockJSON{
		Version:       1,
		TrxID:         1,
		ActiveProfile: "default",
		LoadVimrc:     true,
		LoadGvimrc:    true,
		Repos:         make([]Repos, 0),
		Profiles: []Profile{
			Profile{
				Name:       "default",
				ReposPath:  make([]string, 0),
				LoadVimrc:  true,
				LoadGvimrc: true,
			},
		},
	}
}

func Read() (*LockJSON, error) {
	// Return initial lock.json struct if lockfile does not exist
	lockfile := pathutil.LockJSON()
	if _, err := os.Stat(lockfile); os.IsNotExist(err) {
		return InitialLockJSON(), nil
	}

	// Read lock.json
	bytes, err := ioutil.ReadFile(lockfile)
	if err != nil {
		return nil, err
	}
	var lockJSON LockJSON
	err = json.Unmarshal(bytes, &lockJSON)
	if err != nil {
		return nil, err
	}

	// Validate lock.json
	err = validate(&lockJSON)
	if err != nil {
		return nil, err
	}

	return &lockJSON, nil
}

func validate(lockJSON *LockJSON) error {
	// Validate if missing required keys exist
	err := validateMissing(lockJSON)
	if err != nil {
		return err
	}

	// Validate if duplicate repos[]/path exist
	dup := make(map[string]bool, len(lockJSON.Repos))
	for _, repos := range lockJSON.Repos {
		if _, exists := dup[repos.Path]; exists {
			return errors.New("duplicate repos '" + repos.Path + "'")
		}
		dup[repos.Path] = true
	}

	// Validate if duplicate profiles[]/name exist
	dup = make(map[string]bool, len(lockJSON.Profiles))
	for _, profile := range lockJSON.Profiles {
		if _, exists := dup[profile.Name]; exists {
			return errors.New("duplicate profile '" + profile.Name + "'")
		}
		dup[profile.Name] = true
	}

	// Validate if duplicate profiles[]/repos_path[] exist
	for _, profile := range lockJSON.Profiles {
		dup = make(map[string]bool, len(lockJSON.Profiles)*10)
		for _, reposPath := range profile.ReposPath {
			if _, exists := dup[reposPath]; exists {
				return errors.New("duplicate '" + reposPath + "' (repos_path) in profile '" + profile.Name + "'")
			}
			dup[reposPath] = true
		}
	}

	// Validate if active_profile exists in profiles[]/name
	found := false
	for _, profile := range lockJSON.Profiles {
		if profile.Name == lockJSON.ActiveProfile {
			found = true
			break
		}
	}
	if !found {
		return errors.New("'" + lockJSON.ActiveProfile + "' (active_profile) doesn't exist in profiles")
	}

	// Validate if profiles[]/repos_path[] exists in repos[]/path
	for i, profile := range lockJSON.Profiles {
		for j, reposPath := range profile.ReposPath {
			found := false
			for _, repos := range lockJSON.Repos {
				if reposPath == repos.Path {
					found = true
					break
				}
			}
			if !found {
				return errors.New(
					"'" + reposPath + "' (profiles[" + strconv.Itoa(i) +
						"].repos_path[" + strconv.Itoa(j) + "]) doesn't exist in repos")
			}
		}
	}

	// Validate if repos[]/path exists on filesystem
	// and is a directory
	for i, repos := range lockJSON.Repos {
		fullpath := pathutil.FullReposPathOf(repos.Path)
		if file, err := os.Stat(fullpath); os.IsNotExist(err) {
			return errors.New("'" + fullpath + "' (repos[" + strconv.Itoa(i) + "].path) doesn't exist on filesystem")
		} else if !file.IsDir() {
			return errors.New("'" + fullpath + "' (repos[" + strconv.Itoa(i) + "].path) is not a directory")
		}
	}

	// Validate if trx_id is equal or greater than repos[]/trx_id
	index := -1
	var max int64
	for i, repos := range lockJSON.Repos {
		if max < repos.TrxID {
			index = i
			max = repos.TrxID
		}
	}
	if max > lockJSON.TrxID {
		return errors.New("'" + strconv.FormatInt(max, 10) + "' (repos[" + strconv.Itoa(index) + "].trx_id) " +
			"is greater than '" + strconv.FormatInt(lockJSON.TrxID, 10) + "' (trx_id)")
	}

	return nil
}

func validateMissing(lockJSON *LockJSON) error {
	if lockJSON.Version == 0 {
		return errors.New("missing: version")
	}
	if lockJSON.TrxID == 0 {
		return errors.New("missing: trx_id")
	}
	if lockJSON.Repos == nil {
		return errors.New("missing: repos")
	}
	for i, repos := range lockJSON.Repos {
		if repos.Type == "" {
			return errors.New("missing: repos[" + strconv.Itoa(i) + "].type")
		}
		switch repos.Type {
		case ReposGitType:
			if repos.Version == "" {
				return errors.New("missing: repos[" + strconv.Itoa(i) + "].version")
			}
			fallthrough
		case ReposStaticType:
			if repos.TrxID == 0 {
				return errors.New("missing: repos[" + strconv.Itoa(i) + "].trx_id")
			}
			if repos.Path == "" {
				return errors.New("missing: repos[" + strconv.Itoa(i) + "].path")
			}
		default:
			return errors.New("repos[" + strconv.Itoa(i) + "].type is invalid type: " + string(repos.Type))
		}
	}
	if lockJSON.Profiles == nil {
		return errors.New("missing: profiles")
	}
	for i, profile := range lockJSON.Profiles {
		if profile.Name == "" {
			return errors.New("missing: profile[" + strconv.Itoa(i) + "].name")
		}
		if profile.ReposPath == nil {
			return errors.New("missing: profile[" + strconv.Itoa(i) + "].repos_path")
		}
		for j, reposPath := range profile.ReposPath {
			if reposPath == "" {
				return errors.New("missing: profile[" + strconv.Itoa(i) + "].repos_path[" + strconv.Itoa(j) + "]")
			}
		}
	}
	return nil
}

func (lockJSON *LockJSON) Write() error {
	// Validate lock.json
	err := validate(lockJSON)
	if err != nil {
		return err
	}

	// Mkdir all if lock.json's directory does not exist
	lockfile := pathutil.LockJSON()
	if _, err := os.Stat(filepath.Dir(lockfile)); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(lockfile), 0755)
		if err != nil {
			return err
		}
	}

	// Write to lock.json
	bytes, err := json.MarshalIndent(lockJSON, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(pathutil.LockJSON(), bytes, 0644)
}

func (profs *profiles) FindByName(name string) (*Profile, error) {
	for i, p := range *profs {
		if p.Name == name {
			return &(*profs)[i], nil
		}
	}
	return nil, errors.New("profile '" + name + "' does not exist")
}

func (profs *profiles) FindIndexByName(name string) int {
	for i, p := range *profs {
		if p.Name == name {
			return i
		}
	}
	return -1
}

func (profs *profiles) RemoveAllReposPath(reposPath string) error {
	for i := range *profs {
		for j := range (*profs)[i].ReposPath {
			if (*profs)[i].ReposPath[j] == reposPath {
				(*profs)[i].ReposPath = append(
					(*profs)[i].ReposPath[:j],
					(*profs)[i].ReposPath[j+1:]...,
				)
				return nil
			}
		}
	}
	return errors.New("no matching profiles[]/repos_path[]: " + reposPath)
}

func (reposList *repos) FindByPath(reposPath string) (*Repos, error) {
	for i, repos := range *reposList {
		if repos.Path == reposPath {
			return &(*reposList)[i], nil
		}
	}
	return nil, errors.New("repos '" + reposPath + "' does not exist")
}

func (reposList *repos) RemoveAllByPath(reposPath string) error {
	for i := range *reposList {
		if (*reposList)[i].Path == reposPath {
			*reposList = append((*reposList)[:i], (*reposList)[i+1:]...)
			return nil
		}
	}
	return errors.New("no matching repos[]/path: " + reposPath)
}

func (reposPathList *profReposPath) Contains(reposPath string) bool {
	return reposPathList.IndexOf(reposPath) >= 0
}

func (reposPathList *profReposPath) IndexOf(reposPath string) int {
	for i := range *reposPathList {
		if (*reposPathList)[i] == reposPath {
			return i
		}
	}
	return -1
}

func (lockJSON *LockJSON) GetReposListByProfile(profile *Profile) ([]Repos, error) {
	var reposList []Repos
	for _, reposPath := range profile.ReposPath {
		repos, err := lockJSON.Repos.FindByPath(reposPath)
		if err != nil {
			return nil, err
		}
		reposList = append(reposList, *repos)
	}
	return reposList, nil
}
