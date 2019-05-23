package cmd

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cybera/ccds/internal/languages"
	"github.com/cybera/ccds/internal/paths"
	"github.com/cybera/ccds/internal/templates"
	"github.com/cybera/ccds/internal/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var author, license, language string
var force, nonInteractive bool

var initCmd = &cobra.Command{
	Use:              "init",
	Short:            "Creates a basic data science project skeleton",
	Args:             cobra.ExactArgs(0),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {},
	Run: func(cmd *cobra.Command, args []string) {
		licenses := []string{
			"MIT",
			"BSD-3-Clause",
			"None",
		}

		if viper.GetString("ProjectRoot") != "" {
			log.Fatal("Project has already been initialized")
		}

		projectRoot, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		reader := bufio.NewReader(os.Stdin)

		files, err := ioutil.ReadDir(projectRoot)
		if err != nil {
			log.Fatal(err)
		}

		if len(files) > 0 && !force {
			fmt.Print("This directory is not empty, initialize anyways? [y/N]: ")

			for {
				input := getInput(reader)

				if input == "y" {
					break
				} else if input == "n" || input == "" {
					os.Exit(0)
				}

				fmt.Print("Please answer [y/N]: ")
			}
		}

		viper.Set("ProjectRoot", projectRoot)

		if author == "" {
			fmt.Print("Author (Your name or organization/company/team): ")
			author = getInput(reader)
		}

		if license == "" {
			var choices string

			for i := range licenses {
				choices += strconv.Itoa(i+1) + ", "
			}
			choices = choices[:len(choices)-2]

			fmt.Println("Select your license: ")
			for i, v := range licenses {
				fmt.Println(i+1, "-", v)
			}

			for {
				fmt.Printf("Choose %s: ", choices)
				input := getInput(reader)

				choice, err := strconv.Atoi(input)
				if err == nil && choice > 0 && choice <= len(licenses) {
					license = licenses[choice-1]
					break
				}
			}
		} else if !utils.Contains(licenses, license) {
			log.Fatal("unknown license")
		}

		if language == "" {
			choices := ""

			for i := range languages.Supported {
				choices += strconv.Itoa(i+1) + ", "
			}
			choices = choices[:len(choices)-2]

			fmt.Println("Select your primary language: ")
			for i, v := range languages.Supported {
				fmt.Println(i+1, "-", v)
			}

			for {
				fmt.Printf("Choose %s [1]: ", choices)
				input := getInput(reader)

				if input == "" {
					language = languages.Supported[0]
					break
				}

				choice, err := strconv.Atoi(input)
				if err == nil && choice > 0 && choice <= len(languages.Supported) {
					language = languages.Supported[choice-1]
					break
				}
			}
		} else if !utils.Contains(languages.Supported, language) {
			log.Fatal("unknown language")
		}

		viper.Set("Author", author)
		viper.Set("License", license)
		viper.Set("PrimaryLanguage", language)

		log.Println("Creating project skeleton...")
		if err := createSkeleton(); err != nil {
			log.Fatal(err)
		}

		if err := writeLicense(author, license); err != nil {
			log.Fatal(err)
		}

		log.Println("Initializing git repository...")
		if err := initRepo(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVar(&author, "author", "", "Author name")
	initCmd.Flags().StringVar(&license, "license", "", "Project license")
	initCmd.Flags().StringVar(&language, "language", "", "Which programming language to use")
	initCmd.Flags().BoolVarP(&force, "force", "f", false, "Ignore existing files and directories")
	initCmd.Flags().BoolVarP(&nonInteractive, "non-interactive", "n", false, "Error if any user input is required")
}

func getInput(reader *bufio.Reader) string {
	if nonInteractive {
		log.Fatal("\nerror: input required in non-interactive mode")
	}

	input, _ := reader.ReadString('\n')
	return utils.Chomp(input)
}

func createSkeleton() error {
	projectRoot := viper.GetString("ProjectRoot")
	language := viper.GetString("PrimaryLanguage")
	gitignore := "gitignore/" + language

	// Key is the directory path, value is whether to create a .gitkeep file
	directories := map[string]bool{
		".ccds":             false,
		"data":              false,
		"data/external":     true,
		"data/interim":      true,
		"data/processed":    true,
		"data/raw":          true,
		"docs":              true,
		"models":            true,
		"notebooks":         true,
		"references":        true,
		"reports":           false,
		"reports/figures":   true,
		"src":               false,
		"src/datasets":      true,
		"src/features":      true,
		"src/models":        true,
		"src/scripts":       true,
		"src/visualization": true,
	}

	files := map[string]string{
		gitignore:                   ".gitignore",
		"docker/Dockerfile":         filepath.Join(projectRoot, paths.Dockerfile()),
		"docker/docker-compose.yml": filepath.Join(projectRoot, paths.DockerCompose()),
	}

	for k, v := range languages.InitFiles[language] {
		files[k] = v
	}

	for dir, keep := range directories {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return errors.Wrapf(err, "failed to create directory %s", dir)
		}

		if keep {
			path := filepath.Join(dir, ".gitkeep")
			file, err := os.Create(path)
			if err != nil {
				return errors.Wrapf(err, "failed to create file %s", path)
			}
			file.Close()
		}
	}

	for src, dest := range files {
		if err := templates.Write(src, dest, struct{}{}); err != nil {
			return err
		}
	}

	if err := utils.WriteConfig(); err != nil {
		return err
	}

	return nil
}

func writeLicense(author, license string) error {
	if license == "None" {
		return nil
	}

	src := "licenses/" + license

	data := struct {
		Year, Author string
	}{
		strconv.Itoa(time.Now().Year()),
		author,
	}

	return templates.Write(src, "LICENSE", data)
}

func initRepo() error {
	files, err := ioutil.ReadDir("./")
	if err != nil {
		return errors.Wrap(err, "failed to detect existing git repo")
	}

	for _, f := range files {
		if f.Name() == ".git" && f.IsDir() {
			return errors.New("git repo already exists")
		}
	}

	if _, err := exec.LookPath("git"); err != nil {
		return errors.Wrap(err, "git not found in path")
	}

	if err := exec.Command("git", "init").Run(); err != nil {
		return errors.Wrap(err, "failed to initialize git repo")
	}

	gitAdd(".ccds")
	gitCommit("Add ccds config directory")
	gitAdd(".gitignore", "LICENSE")
	gitCommit("Add standard repo files")
	gitAdd("Dockerfile", "docker-compose.yml")
	gitCommit("Add Docker configuration for Jupyter")
	gitAdd("data")
	gitCommit("Add directory for storing datasets")
	gitAdd("docs")
	gitCommit("Add directory for storing documentation")
	gitAdd("models")
	gitCommit("Add directory for storing models")
	gitAdd("notebooks")
	gitCommit("Add directory for storing notebooks")
	gitAdd("references")
	gitCommit("Add directory for storing references")
	gitAdd("reports")
	gitCommit("Add directory for storing reports")
	gitAdd("src")
	gitCommit("Add directory for storing source code")

	return nil
}

func gitAdd(paths ...string) error {
	args := append([]string{"add"}, paths...)
	return exec.Command("git", args...).Run()
}

func gitCommit(message string) error {
	return exec.Command("git", "commit", "-m", message).Run()
}
