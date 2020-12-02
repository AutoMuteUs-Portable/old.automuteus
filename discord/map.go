package discord

import (
	"fmt"
	"strings"
	"errors"
	"net/url"
	"log"
)

type MapType struct {
	Name string
	MapImage MapImage
}

type MapImage struct {
	Simple string
	Detailed string
}

func (m *MapType) String() string {
    return m.Name
}

const BaseMapURL = "https://github.com/ShawnHardwick/automuteus/blob/feature/map_command/assets/maps/"

func NewMapFromName(name string) (*MapType, error) {
    switch strings.ToLower(name) {
	case "the_skeld", "skeld":
		name = "skeld"
	case "mira_hq", "mira hq", "mirahq":
		name = "mira_hq"
	case "polus":
		name = "polus"
	default:
		return nil, errors.New(fmt.Sprintf("Invalid map name: %s", name))
	}

	base, err := url.Parse(BaseMapURL)
    if err != nil {
        log.Println(err)
	}

	simpleURL, err := base.Parse(name + ".png?raw=true")
	if err != nil {
        log.Println(err)
	}

	detailedURL, err := base.Parse(name + "_detailed.png?raw=true")
	if err != nil {
        log.Println(err)
	}

	mapImage := MapImage{
		Simple:   simpleURL.String(),
		Detailed: detailedURL.String(),
	}

	return &MapType{Name: name, MapImage: mapImage}, nil
}

