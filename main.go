package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/gookit/color"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

var Cfg *Config

type Palette struct {
	Fg               string   `yaml:"fg"`
	Bg               string   `yaml:"bg"`
	RegexpStringList []string `yaml:"regexp"`
	RegexpList       []*regexp.Regexp
}
type Config struct {
	Palette []Palette
	Match   []struct {
		RegexpStringList []string `yaml:"regexp"`
	} `yaml:"match"`
}

func NewConfig(configPath string, regexpSlice []string) (*Config, error) {
	c := &Config{}
	if err := c.validateConfigPath(configPath); err == nil {
		if err := c.fromFile(configPath); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if len(c.Palette) == 0 {
		c.setDefaultPaletteColors()
	}

	pIndex := 0
	for _, m := range regexpSlice {
		c.Palette[pIndex].RegexpStringList = append(c.Palette[pIndex].RegexpStringList, m)
		pIndex++
		if pIndex > len(c.Palette)-1 {
			pIndex = 0
		}
	}

	for _, m := range c.Match {
		c.Palette[pIndex].RegexpStringList = append(c.Palette[pIndex].RegexpStringList, m.RegexpStringList...)
		pIndex++
		if pIndex > len(c.Palette)-1 {
			pIndex = 0
		}
	}
	if err := c.compileRegexps(); err != nil {
		return nil, errors.WithStack(err)
	}

	return c, nil
}

func (c *Config) fromFile(configPath string) error {
	file, err := os.Open(configPath)
	if err != nil {
		return errors.WithStack(err)
	}
	defer file.Close()
	d := yaml.NewDecoder(file)
	if err := d.Decode(&c); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (c *Config) setDefaultPaletteColors() {
	for _, v := range []string{
		"#089400",
		"#1b96f3",
		"#ef0195",
		"#fcdf87",
		"#f68741",
		"#8CCBEA",
		"#A4E57E",
		"#FFDB72",
		"#ad4c35",
		"#FF7272",
		"#FFB3FF",
		"#9999FF",
		// "#320E3B",
		"#4C2A85",
		"#6B7FD7",
		"#BCEDF6",
		"#DDFBD2",
		"#D664BE",
		"#DF99F0",
		"#B191FF",
		"#F4BFDB",
		"#FFE9F3",
		"#87BAAB",
	} {
		c.Palette = append(c.Palette, Palette{
			"", v, nil, nil,
		})
	}
}

func (c Config) validateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return errors.WithStack(err)
	}
	if s.IsDir() {
		return errors.WithStack(err)
	}
	return nil
}

func (c Config) compileRegexps() (err error) {
	for i, p := range c.Palette {
		c.Palette[i].RegexpList = make([]*regexp.Regexp, len(p.RegexpStringList))
		for j, r := range p.RegexpStringList {
			if c.Palette[i].RegexpList[j], err = regexp.Compile(r); err != nil {
				return errors.Wrapf(err, "WROING REGEXP '%s'", r)
			}
		}
	}
	return nil
}

func processLine(line string) string {
	if line == "" {
		return line
	}
	palette2indexes := make(map[int][][]int)
	for i, p := range Cfg.Palette {
		for _, r := range p.RegexpList {
			palette2indexes[i] = append(palette2indexes[i], r.FindAllStringIndex(line, -1)...)
		}
	}
	indexes := []int{0, len(line)}
	for _, ind := range palette2indexes {
		for _, interval := range ind {
			indexes = append(indexes, interval...)
		}
	}
	indexes = funk.UniqInt(indexes)
	sort.Ints(indexes)

	var b strings.Builder
	for i := 0; i < len(indexes)-1; i++ {
		left, right := indexes[i], indexes[i+1]
		paletteList := []int{}
		for paletteI, ind := range palette2indexes {
			for _, v := range ind {
				if (v[0] <= left && left <= v[1]) && (v[0] <= right && right <= v[1]) {
					paletteList = append(paletteList, paletteI)
				}
			}
		}
		if len(paletteList) == 0 {
			b.WriteString(line[left:right])
			continue
		}

		palette := Cfg.Palette[funk.MaxInt(paletteList).(int)]
		c := ""
		if palette.Bg == "" && palette.Fg == "" {
			c = color.HEXStyle("#ff3333").String()
		} else if palette.Bg == "" && palette.Fg != "" {
			c = color.HEXStyle(palette.Fg).String()
		} else if palette.Bg != "" && palette.Fg == "" {
			//TODO: invert maybe
			c = color.HEXStyle("#000000", palette.Bg).String()
		} else {
			c = color.HEXStyle(palette.Fg, palette.Bg).String()
		}
		b.WriteString(color.RenderString(c, line[left:right]))
	}
	return b.String()
}

func main() {
	app := &cli.App{
		// Name:  "cmatch",
		// Usage: "",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Usage:    "path to config file",
				Value:    "./config.yml",
				Required: false,
				// DefaultText: "./config.yml",
			},
			&cli.StringSliceFlag{
				Name:     "regexp",
				Aliases:  []string{"r"},
				Usage:    "-r regexp1 -r regexp2",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			var err error
			Cfg, err = NewConfig(c.String("config"), c.StringSlice("regexp"))
			if err != nil {
				return errors.WithStack(err)
			}

			reader := bufio.NewReader(os.Stdin)
			var line string
			for {
				if line, err = reader.ReadString('\n'); err != nil {
					break
				}
				fmt.Printf("%s\n", processLine(line[:len(line)-1]))
			}
			if err != io.EOF {
				return errors.WithStack(err)
			}
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
		// fmt.Printf("%+v\n", err)
	}
}
