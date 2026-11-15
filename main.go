package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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
	if !slices.Contains([]string{"build", "install", "uninstall", "update", "add"}, os.Args[1]) {
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
	err := os.Chdir(getHomePath(".local/share/gitop/" + repo.Name))
	if err != nil {
		log.Fatal("Could not change to repo dir maybe it was added in bad ways?")
	}
	fmt.Println("Doing Build for " + buildType.FileName)
	split := strings.Split(buildType.BuildCmds.Build, " ")
	cmd := exec.Command(split[0], split[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal("Error on build: " + err.Error())
	}
}

func doAdd(url string, buildTypes []BuildType) Repo {
	fmt.Println("Adding and cloning: " + url)
	err := os.Chdir(getHomePath(".local/share/gitop"))
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
	entried, err := os.ReadDir(getHomePath(filepath.Join(".local/share/gitop/", cleanURL)))
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
	cmd = exec.Command("bash", "-c", "git -C "+getHomePath(".local/share/gitop/"+cleanURL)+" rev-parse HEAD")
	version, err := cmd.Output()
	fmt.Printf("%s\n", string(version))
	if err != nil {
		log.Fatal("Error while getting version: " + err.Error())
	}
	repo := Repo{
		Name:          cleanURL,
		URL:           url,
		BuildTypeFile: buildFile,
		Version:       string(version),
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
	err := os.Chdir(getHomePath(".local/share/gitop/" + repo.Name))
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

func doUninstall(repo Repo, buildType BuildType, repos []Repo) []Repo {
	cmd := exec.Command("bash", "-c", buildType.BuildCmds.Uninstall)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal("Error while uninstaling: " + err.Error())
	}
	cmd = exec.Command("rm", "-rf", getHomePath(".local/share/gitop/"+repo.Name))
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

func main() {
	if len(os.Args) < 3 {
		prettyPrintInfo()
		os.Exit(0)
	}
	for _, arg := range os.Args {
		fmt.Printf("%s\n", arg)
	}
	conf, err := createConf(getHomePath(".config/gitop/config.json"))
	if err != nil {
		log.Fatalf("Error while making ctx %s\n", err.Error())
	}
	task := validateArgs()

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
		err = os.WriteFile(getHomePath(".config/gitop/config.json"), out, 0o644)
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
		err = os.WriteFile(getHomePath(".config/gitop/config.json"), out, 0o644)
		if err != nil {
			log.Fatal("Could not write config to file: " + err.Error())
		}
	}
}
