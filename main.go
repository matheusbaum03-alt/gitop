package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
)

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
		log.Fatal("Could not change to repo dir maybe it was added in a bad way? Try clonning manualy")
	}
	fmt.Printf("Doing Build for %s using a %s\n", repo.Name, repo.BuildType.FileName)
	cmd := exec.Command("bash", "-c", repo.BuildType.BuildCmds.Build)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal("Error on build: " + err.Error())
	}
}

func doAdd(url string, buildTypes []BuildType) Repo {
	fmt.Println("Adding and cloning: " + url)
	err := os.Chdir(getCachePath(""))
	if err != nil {
		log.Fatal("could not go to ~/.local/share/gitop make it if needed")
	}
	cmd := exec.Command("git", "clone", "--depth", "1", url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal("error while cloning: " + err.Error())
	}
	cleanURL := strings.TrimSuffix(url, ".git")
	cleanURL = path.Base(cleanURL)
	entried, err := os.ReadDir(getCachePath(cleanURL))
	if err != nil {
		log.Fatal("Error reading repo: " + err.Error())
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
		fmt.Println("No matching build type found returning empty to add use: gitop chBuild " + cleanURL + " <path to a buildType in json>")
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
	err := os.Chdir(getCachePath(repo.Name))
	if err != nil {
		log.Fatal("Could not go to the repo path: " + err.Error())
	}
	cmd := exec.Command("bash", "-c", repo.BuildType.BuildCmds.Install)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal("Error while instaling: " + err.Error())
	}
}

func getVersion(pathU string) string {
	cmd := exec.Command("bash", "-c", "git -C "+getCachePath(pathU)+" rev-parse HEAD")
	cmd.Stderr = os.Stderr
	version, err := cmd.Output()
	if err != nil {
		log.Fatal("Getting version returned: " + err.Error())
	}
	return string(version)
}

func doUninstall(repo Repo, repos []Repo) []Repo {
	cmd := exec.Command("bash", "-c", repo.BuildType.BuildCmds.Uninstall)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal("Error while uninstaling: " + err.Error())
	}
	cmd = exec.Command("rm", "-rf", getCachePath(repo.Name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal("Error Deleting repo please do it yourself")
	}
	toExclude := -1
	for i, re := range repos {
		if repo == re {
			toExclude = i
			break
		}
	}
	if toExclude == -1 {
		log.Fatal("Could not find repo in repos that were given how did this happen I have no idea")
	}
	repos = append(repos[:toExclude], repos[toExclude+1:]...)
	return repos
}

func doUpdate(repo Repo) Repo {
	err := os.Chdir(getCachePath(repo.Name))
	if err != nil {
		log.Fatal("Check if repo exists maybe a failed deletion or uncloned repo look: " + err.Error())
	}
	fmt.Println("Updating: " + repo.Name)
	oldVersion := getVersion(repo.Name)
	cmd := exec.Command("bash", "-c", "git fetch && git reset --hard && git clean -fd")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		log.Fatal("Error while updating repo: " + err.Error())
	}
	newVersion := getVersion(repo.Name)
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

func main() {
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
		log.Fatalf("Error while making ctx %s\n", err.Error())
	}
	if task == "build" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			log.Fatal("Could not find repo")
		}
		doBuild(repo)
	}
	if task == "add" {
		repo := doAdd(os.Args[2], conf.BuildTypes)
		for _, re := range conf.Repos {
			if re == repo {
				log.Fatal("Repo was already added")
			}
		}
		conf.Repos = append(conf.Repos, repo)
		out, err := json.Marshal(conf)
		if err != nil {
			log.Fatal("Could Not encode config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			log.Fatal("Could Not write encoded config: " + err.Error())
		}
	}
	if task == "install" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			log.Fatal("Could not find repo")
		}
		doInstall(repo)
	}
	if task == "uninstall" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			log.Fatal("Could not find repo")
		}
		conf.Repos = doUninstall(repo, conf.Repos)
		out, err := json.Marshal(conf)
		if err != nil {
			log.Fatal("Error encoding config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			log.Fatal("Could not write config to file: " + err.Error())
		}
	}
	if task == "update" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			log.Fatal("Repo not Found")
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
			log.Fatal("Error encoding config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			log.Fatal("Error while writting config: " + err.Error())
		}
	}
	if task == "updateAll" {
		conf = doUpdateAll(conf)
		out, err := json.Marshal(conf)
		if err != nil {
			log.Fatal("Error encoding config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			log.Fatal("Error while writting config: " + err.Error())
		}
	}
	if task == "chBuild" {
		if len(os.Args) != 4 {
			log.Fatal("To chBuild use: gitop chBuild <repo> <path to a json that is a buildType>")
		}
		out, err := os.ReadFile(os.Args[3])
		if err != nil {
			log.Fatal("Error opening new buildType: " + err.Error())
		}
		var build BuildType
		err = json.Unmarshal(out, &build)
		if err != nil {
			log.Fatal("Error while decoding build type: " + err.Error())
		}
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		conf.Repos = temp
		if Nfound {
			log.Fatal("Repo not found")
		}
		conf.Repos = chBuild(repo, build, conf.Repos)
		out, err = json.Marshal(conf)
		if err != nil {
			log.Fatal("Error encoding config: " + err.Error())
		}
		err = os.WriteFile(getConfigPath(), out, 0o644)
		if err != nil {
			log.Fatal("Error while writting config: " + err.Error())
		}
	}
	if task == "buildInstall" {
		repo, temp, Nfound := getRepoNParse(conf.Repos)
		if Nfound {
			log.Fatal("Could not find repo")
		}
		conf.Repos = temp
		doBuild(repo)
		doInstall(repo)
	}
}
