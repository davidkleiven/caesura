package api

import "github.com/davidkleiven/caesura/pkg"

var saxophones = []string{
	"Soprano Saxophone",
	"Alto Saxophone",
	"Tenor Saxophone",
	"Baritone Saxophone",
	"Bass Saxophone",
}

var brass = []string{
	"Piccolo Trumpet",
	"Trumpet",
	"Cornet Eb",
	"Cornet",
	"Baritone",
	"Euphonium",
	"Trombone",
	"Tuba",
}

var choir = []string{
	"Soprano",
	"Alto",
	"Tenor",
	"Bass",
}

var percussion = []string{
	"Cymbals",
	"Drum kit",
	"Marimba",
	"Percussion",
	"Vibraphone",
	"Xylophone",
}

var stringInstruments = []string{
	"Violin",
	"Viola",
	"Cello",
	"Contrabass",
}

var clarinets = []string{
	"Eb Clarinet",
	"Clarinet",
	"Alto Clarinet",
	"Bass Clarinet",
}

var otherReeds = []string{
	"Oboe",
	"Basoon",
}

var flutes = []string{
	"Piccolo",
	"Flute",
	"Alto Flute",
	"Bass Flute",
}

func allInstruments() []string {
	var allInstruments []string
	allInstruments = append(allInstruments, clarinets...)
	allInstruments = append(allInstruments, brass...)
	allInstruments = append(allInstruments, saxophones...)
	allInstruments = append(allInstruments, choir...)
	allInstruments = append(allInstruments, flutes...)
	allInstruments = append(allInstruments, stringInstruments...)
	allInstruments = append(allInstruments, otherReeds...)
	allInstruments = append(allInstruments, percussion...)
	return allInstruments
}

func Instruments(token string) []string {
	return pkg.FilterList(allInstruments(), token)
}
