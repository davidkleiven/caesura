package main

import (
	"fmt"
	"os"

	"github.com/davidkleiven/caesura/pkg"
	"gopkg.in/yaml.v2"
)

func main() {
	defaultConfig := pkg.NewDefaultConfig()
	outfile := "config.yml"

	out, err := yaml.Marshal(defaultConfig)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	f, err := os.Create(outfile)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
	defer f.Close()
	f.Write(out)
	fmt.Printf("Configuration template written to: %s\n", outfile)
}
