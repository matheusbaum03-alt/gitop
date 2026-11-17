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
	Name          string `json:"name"`
	URL           string `json:"url"`
	BuildTypeFile string `json:"buildTypeFile"`
	// The sha256 of the commit
	Version string `json:"version"`
}

type BuildCmd struct {
	Build     string `json:"build"`
	Install   string `json:"install"`
	Uninstall string `json:"uninstall"`
}

type BuildType struct {
	FileName  string `json:"fileName"`
	BuildCmds BuildCmd
}

type config struct {
	Repos      []Repo      `json:"repos"`
	BuildTypes []BuildType `json:"buildTypes"`
}

func validateArgs() string {
	if !slices.Contains([]string{"build", "install", "uninstall", "update", "add", "updateAll"}, os.Args[1]) {
		fmt.Printf("invalid option: you put %s\n can be: build, install, uninstall, update or add\n", os.Args[1])
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

func getBuild(repo Repo, buildTypes []BuildType) BuildType {
	var res BuildType
	for _, buildType := range buildTypes {
		if buildType.FileName == repo.BuildTypeFile {
			parseCmds(&buildType.BuildCmds, repo.Name)
			res = buildType
		}
	}
	return res
}

func doBuild(repo Repo, buildType BuildType) {
	err := os.Chdir(getCachePath(repo.Name))
	if err != nil {
		log.Fatal("Could not change to repo dir maybe it was added in a bad way? Try clonning manualy")
	}
	fmt.Println("Doing Build for " + buildType.FileName)
	cmd := exec.Command("bash", "-c", buildType.BuildCmds.Build)
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
	cmd := exec.Command("git", "clone", url)
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
	var buildFile string
	for _, entry := range entried {
		for _, buildType := range buildTypes {
			if entry.Name() == buildType.FileName && !entry.IsDir() {
				buildFile = entry.Name()
			}
		}
	}
	version := getVersion(cleanURL)
	repo := Repo{
		Name:          cleanURL,
		URL:           url,
		BuildTypeFile: buildFile,
		Version:       version,
	}
	return repo
}

func getRepo(repos []Repo) (Repo, bool) {
	for _, repo := range repos {
		if repo.Name == os.Args[2] {
			return repo, false
		}
	}
	return Repo{}, true
}

func doInstall(repo Repo, buildType BuildType) {
	err := os.Chdir(getCachePath(repo.Name))
	if err != nil {
		log.Fatal("Could not chnage to the repo path: " + err.Error())
	}
	cmd := exec.Command("bash", "-c", buildType.BuildCmds.Install)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal("Error while instaling: " + err.Error())
	}
}

func getVersion(pathU string) string {
	fmt.Println(pathU)
	cmd := exec.Command("bash", "-c", "git -C "+getCachePath(pathU)+" rev-parse HEAD")
	cmd.Stderr = os.Stderr
	version, err := cmd.Output()
	if err != nil {
		log.Fatal("Getting version returned: " + err.Error())
	}
	return string(version)
}

func doUninstall(repo Repo, buildType BuildType, repos []Repo) []Repo {
	cmd := exec.Command("bash", "-c", buildType.BuildCmds.Uninstall)
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
	repos = append(repos[:toExclude], repos[toExclude+1:]...)
	return repos
}

func doUpdate(repo Repo, buildType BuildType) Repo {
	err := os.Chdir(getCachePath(repo.Name))
	if err != nil {
		log.Fatal("Check if repo exists maybe a failed deletion or uncloned repo look: " + err.Error())
	}
	fmt.Println(repo.Name)
	oldVersion := getVersion(repo.Name)
	cmd := exec.Command("bash", "-c", fmt.Sprintf("rm -rfv %s", getCachePath(repo.Name)))
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal("Error deleting original repo may need to reclone manualy: " + err.Error())
	}
	err = os.Chdir(getCachePath(""))
	if err != nil {
		log.Fatal("Could not go to cache something terrible happened " + err.Error())
	}
	cmd = exec.Command("git", "clone", repo.URL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal("Error while Reclonning repo" + err.Error())
	}
	newVersion := getVersion(repo.Name)
	if newVersion == oldVersion {
		fmt.Println("nothing to do alredy updated")
		return repo
	}
	repo.Version = newVersion
	doBuild(repo, buildType)
	doInstall(repo, buildType)
	return repo
}

func doUpdateAll(conf config) config {
	for i, repo := range conf.Repos {
		build := getBuild(repo, conf.BuildTypes)
		repoN := doUpdate(repo, build)
		conf.Repos[i] = repoN
	}
	return conf
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
		repo, Nfound := getRepo(conf.Repos)
		if Nfound {
			log.Fatal("Could not find repo")
		}
		buildType := getBuild(repo, conf.BuildTypes)
		doBuild(repo, buildType)
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
		repo, Nfound := getRepo(conf.Repos)
		if Nfound {
			log.Fatal("Could not find repo")
		}
		buildType := getBuild(repo, conf.BuildTypes)
		doInstall(repo, buildType)
	}
	if task == "uninstall" {
		repo, Nfound := getRepo(conf.Repos)
		if Nfound {
			log.Fatal("Could not find repo")
		}
		buildType := getBuild(repo, conf.BuildTypes)
		conf.Repos = doUninstall(repo, buildType, conf.Repos)
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
		repo, Nfound := getRepo(conf.Repos)
		if Nfound {
			log.Fatal("Repo not Found")
		}
		buildType := getBuild(repo, conf.BuildTypes)
		repoN := doUpdate(repo, buildType)
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
}
