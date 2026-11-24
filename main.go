package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	"gitop/snapshots"
)

type DeltaEntries struct {
	RepoName string            `json:"repoName"`
	Entries  []snapshots.Entry `json:"entries"`
}

type Delta struct {
	DeltaEntries []DeltaEntries `json:"deltaEntries"`
}

type Repo struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	BuildType BuildType `json:"buildType"`
	// The sha256 of the commit
	Version string `json:"version"`
}

type BuildCmd struct {
	Build     string `json:"build"`
	Install   string `json:"install"`
	Uninstall string `json:"uninstall"`
}

type BuildType struct {
	FileName  string   `json:"fileName"`
	BuildCmds BuildCmd `json:"buildCmds"`
}

type config struct {
	Repos      []Repo      `json:"repos"`
	BuildTypes []BuildType `json:"buildTypes"`
}

func validateArgs() string {
	if !slices.Contains([]string{"build", "install", "uninstall", "update", "add", "updateAll", "chBuild", "buildInstall"}, os.Args[1]) {
		fmt.Printf("invalid option: you put %s\n can be: build, install, uninstall, update, updateAll, chBuild, buildInstall or add\n", os.Args[1])
		os.Exit(1)
	}
	return os.Args[1]
}

func parseCmds(cmds *BuildCmd, name string) {
	cmds.Build = strings.ReplaceAll(cmds.Build, "<name>", name)
	cmds.Install = strings.ReplaceAll(cmds.Install, "<name>", name)
	cmds.Uninstall = strings.ReplaceAll(cmds.Uninstall, "<name>", name)
}

func createConf(configPath string) (config, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return config{}, err
	}
	var conf config
	err = json.Unmarshal(content, &conf)
	if err != nil {
		return config{}, err
	}
	return conf, nil
}

func doBuild(repo Repo) {
	err := os.Chdir(getCachePath(repo.Name))
	if err != nil {
		LogF("Could not change to repo dir maybe it was added in a bad way? Try clonning manualy")
	}
	fmt.Printf("Doing Build for %s using a %s\n", repo.Name, repo.BuildType.FileName)
	cmd := exec.Command("bash", "-c", repo.BuildType.BuildCmds.Build)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		LogF("Error on build: " + err.Error())
	}
}

func doAdd(url string, buildTypes []BuildType, deep bool) Repo {
	fmt.Println("Adding and cloning: " + url)
	err := os.Chdir(getCachePath(""))
	if err != nil {
		LogF("could not go to ~/.local/share/gitop make it if needed")
	}
	var cmd *exec.Cmd
	if deep {
		cmd = exec.Command("git", "clone", "--depth", "1", url)
	} else {
		cmd = exec.Command("git", "clone", url)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		LogF("error while cloning: " + err.Error())
	}
	cleanURL := strings.TrimSuffix(url, ".git")
	cleanURL = path.Base(cleanURL)
	entried, err := os.ReadDir(getCachePath(cleanURL))
	if err != nil {
		LogF("Error reading repo: " + err.Error())
	}
	var buildType BuildType
	for _, entry := range entried {
		for _, buildTypeL := range buildTypes {
			if entry.Name() == buildTypeL.FileName && !entry.IsDir() {
				buildType = buildTypeL
			}
		}
	}
	if buildType.FileName == "" {
		fmt.Println("No matching build type found returning empty to add use: gitop chBuild " + cleanURL + " <path to a buildType in json> to insert a custom one bash scripts work")
	}
	version := getVersion(cleanURL)
	repo := Repo{
		Name:      cleanURL,
		URL:       url,
		BuildType: buildType,
		Version:   version,
	}
	return repo
}

func getRepoNParse(repos []Repo) (Repo, []Repo, bool) {
	for i, repo := range repos {
		if repo.Name == os.Args[2] {
			parseCmds(&repo.BuildType.BuildCmds, repo.Name)
			parseCmds(&repos[i].BuildType.BuildCmds, repo.Name)
			return repo, repos, false
		}
	}
	return Repo{}, []Repo{{}}, true
}

func doInstall(repo Repo) {
	var delta Delta
	out, err := os.ReadFile(getCachePath("delta.json"))
	if err != nil {
		LogF("Error reading delta.json make it with touch ~/.local/share/gitop/delta.json")
	}
	err = json.Unmarshal(out, &delta)
	if err != nil {
		LogF("error with Unmarshal: " + err.Error())
	}
	var entries []snapshots.Entry
	for i, deltaEntry := range delta.DeltaEntries {
		if repo.Name == deltaEntry.RepoName {
			fmt.Println("Warning: " + repo.Name + " already installed reinstalling")
			entries = deltaEntry.Entries
			delta.DeltaEntries = append(delta.DeltaEntries[:i], delta.DeltaEntries[i+1:]...)
			break
		}
	}
	// 1: get first snapshot

	oldS := snapshots.GetMapDir(getLocal())

	// 2: install command
	err = os.Chdir(getCachePath(repo.Name))
	if err != nil {
		LogF("Could not go to the repo path: " + err.Error())
	}
	cmd := exec.Command("bash", "-c", repo.BuildType.BuildCmds.Install)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		LogF("Error while instaling: " + err.Error())
	}
	// 3: new snapshot
	newS := snapshots.GetMapDir(getLocal())
	// 4: compare snapshot
	entriesfN := snapshots.GetDelta(oldS, newS)
	// 4.5: make delta
	var deltaEntry DeltaEntries
	if entries != nil {
		deltaEntry = DeltaEntries{
			RepoName: repo.Name,
			Entries:  entries,
		}
	} else {
		deltaEntry = DeltaEntries{
			RepoName: repo.Name,
			Entries:  entriesfN,
		}
	}
	delta.DeltaEntries = append(delta.DeltaEntries, deltaEntry)
	// 5: write delta
	file, err := os.Create(getCachePath("delta.json"))
	if err != nil {
		LogF("Error opening delta.json: " + err.Error())
	}
	out, err = json.Marshal(delta)
	if err != nil {
		LogF("Error Marshal delta: " + err.Error())
	}
	_, err = file.Write(out)
	if err != nil {
		LogF("Error writting: " + err.Error())
	}
	err = file.Close()
	if err != nil {
		fmt.Println("Error closing neglegible: " + err.Error())
	}
}

func getVersion(pathU string) string {
	cmd := exec.Command("bash", "-c", "git -C "+getCachePath(pathU)+" rev-parse HEAD")
	cmd.Stderr = os.Stderr
	version, err := cmd.Output()
	if err != nil {
		LogF("Getting version returned: " + err.Error())
	}
	return string(version)
}

func doUninstall(repo Repo, repos []Repo, legacy bool) []Repo {
	var delta Delta
	out, err := os.ReadFile(getCachePath("delta.json"))
	if err != nil {
		LogF("Error reading delta.json make it with touch ~/.local/share/gitop/delta.json")
	}
	err = json.Unmarshal(out, &delta)
	if err != nil {
		LogF("error with Unmarshal: " + err.Error())
	}
	deltaToRemove := -1
	var targetEntries []snapshots.Entry
	for i, deltaEntry := range delta.DeltaEntries {
		if repo.Name == deltaEntry.RepoName {
			targetEntries = deltaEntry.Entries
			deltaToRemove = i
			break
		}
	}

	err = os.Chdir(getCachePath(repo.Name))
	if err != nil {
		LogF("error going to repo")
	}
	if legacy {
		cmd := exec.Command("bash", "-c", repo.BuildType.BuildCmds.Uninstall)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			LogF("error while uninstaling: " + err.Error())
		}

		err = os.Chdir("..")
		if err != nil {
			LogF("Error leaving repo")
		}
	} else {
		err = os.Chdir(getLocal())
		if err != nil {
			LogF("going to local")
		}
		for _, entry := range targetEntries {
			if entry.Type == "dir" {
				fmt.Println("Ignoring directory")
				continue
			} else if entry.Type == "file" {
				fmt.Println("Removing: " + entry.Path)
				if err = os.Remove(entry.Path); err != nil {
					LogF("Error removing: " + err.Error())
				}
			} else {
				fmt.Println("Unknown entry type however coded this was tripping")
			}
		}
	}
	cmd := exec.Command("rm", "-rf", getCachePath(repo.Name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		LogF("Error Deleting repo please do it yourself")
	}
	toExclude := -1
	for i, re := range repos {
		if repo == re {
			toExclude = i
			break
		}
	}
	if toExclude == -1 {
		LogF("Could not find repo in repos that were given how did this happen I have no idea")
	}
	repos = append(repos[:toExclude], repos[toExclude+1:]...)

	// Write delta without target

	if deltaToRemove == -1 {
		fmt.Println("Your repo was not installed in the first place just cleaning then")
		return repos
	}
	delta.DeltaEntries = append(delta.DeltaEntries[:deltaToRemove], delta.DeltaEntries[deltaToRemove+1:]...)
	out, err = json.Marshal(delta)
	if err != nil {
		LogF("Error Marshal: " + err.Error())
	}
	err = os.WriteFile(getCachePath("delta.json"), out, 0o644)
	if err != nil {
		LogF("Error Writting delta.json: " + err.Error())
	}
	return repos
}

func doUpdate(repo Repo) Repo {
	err := os.Chdir(getCachePath(repo.Name))
	if err != nil {
		LogF("Check if repo exists maybe a uncloned repo look: " + err.Error())
	}
	fmt.Println("Updating: " + repo.Name)
	oldVersion := getVersion(repo.Name)
	fmt.Println("old: " + oldVersion)
	cmd := exec.Command("bash", "-c", "git pull --force && git clean -fd")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		LogF("Error while updating repo: " + err.Error())
	}
	newVersion := getVersion(repo.Name)
	fmt.Println("new: " + newVersion)
	if newVersion == oldVersion {
		fmt.Println("nothing to do alredy updated")
		return repo
	}
	repo.Version = newVersion
	doBuild(repo)
	doInstall(repo)
	return repo
}

func doUpdateAll(conf config) config {
	for i, repo := range conf.Repos {
		parseCmds(&repo.BuildType.BuildCmds, repo.Name)
		repoN := doUpdate(repo)
		conf.Repos[i] = repoN
	}
	return conf
}

func chBuild(repo Repo, newBuild BuildType, repos []Repo) []Repo {
	var toChange int
	for i, re := range repos {
		if re == repo {
			toChange = i
			break
		}
	}
	repo.BuildType = newBuild
	repos[toChange] = repo
	return repos
}

func LogF(msg string) {
	fmt.Println(msg)
	err := os.Remove(getCachePath("lock.lock"))
	if err != nil {
		log.Fatal("Error removing check if is there")
	}
	os.Exit(1)
}

func lockExists() bool {
	_, err := os.Stat(getCachePath("lock.lock"))
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func main() {
	if lockExists() {
		log.Fatal("Error lock exists wait for the other gitop process to end or weird path/permissions")
	}
	t, err := os.Create(getCachePath("lock.lock"))
	if err != nil {
		LogF("Could not create lock: " + err.Error())
	}
	err = t.Close()
	if err != nil {
		fmt.Println("Error closing lock ignorable: " + err.Error())
	}

	if len(os.Args) <= 2 {
		if len(os.Args) == 1 {
			prettyPrintInfo()
			os.Exit(0)
		} else if os.Args[1] != "updateAll" {
			prettyPrintInfo()
			os.Exit(0)
		}
	}
	task := validateArgs()
	conf, err := createConf(getConfigPath())
	if err != nil {
		LogF("Error while making ctx: " + err.Error())
	}
	if task == "build" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			LogF("Could not find repo")
		}
		doBuild(repo)
	}
	if task == "add" {
		deep := flag.Bool("d", false, "whether to do a deep copy or not")
		flag.Parse()
		repo := doAdd(os.Args[2], conf.BuildTypes, *deep)
		for _, re := range conf.Repos {
			if re == repo {
				LogF("Repo was already added")
			}
		}
		conf.Repos = append(conf.Repos, repo)
		out, err := json.Marshal(conf)
		if err != nil {
			LogF("Could Not encode config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			LogF("Could Not write encoded config: " + err.Error())
		}
	}
	if task == "install" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			LogF("Could not find repo")
		}
		doInstall(repo)
	}
	if task == "uninstall" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			LogF("Could not find repo")
		}
		legacy := flag.Bool("l", false, "uses legacy uninstall")
		flag.Parse()
		conf.Repos = doUninstall(repo, conf.Repos, *legacy)

		out, err := json.Marshal(conf)
		if err != nil {
			LogF("Error encoding config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			LogF("Could not write config to file: " + err.Error())
		}
	}
	if task == "update" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			LogF("Repo not Found")
		}
		repoN := doUpdate(repo)
		for i, re := range conf.Repos {
			if re == repo {
				conf.Repos[i] = repoN
				break
			}
		}
		out, err := json.Marshal(conf)
		if err != nil {
			LogF("Error encoding config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			LogF("Error while writting config: " + err.Error())
		}
	}
	if task == "updateAll" {
		conf = doUpdateAll(conf)
		out, err := json.Marshal(conf)
		if err != nil {
			LogF("Error encoding config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			LogF("Error while writting config: " + err.Error())
		}
	}
	if task == "chBuild" {
		if len(os.Args) != 4 {
			LogF("To chBuild use: gitop chBuild <repo> <path to a json that is a buildType>")
		}
		out, err := os.ReadFile(os.Args[3])
		if err != nil {
			LogF("Error opening new buildType: " + err.Error())
		}
		var build BuildType
		err = json.Unmarshal(out, &build)
		if err != nil {
			LogF("Error while decoding build type: " + err.Error())
		}
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			LogF("Repo not found")
		}
		conf.Repos = chBuild(repo, build, conf.Repos)
		out, err = json.Marshal(conf)
		if err != nil {
			LogF("Error encoding config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			LogF("Error while writting config: " + err.Error())
		}
	}
	if task == "buildInstall" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		if Nfound {
			LogF("Could not find repo")
		}
		conf.Repos = temp
		doBuild(repo)
		doInstall(repo)
	}
	err = os.Remove(getCachePath("lock.lock"))
	if err != nil {
		log.Fatal("Error removing check if is there")
	}
}
