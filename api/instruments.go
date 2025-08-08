package api

import "github.com/davidkleiven/caesura/pkg"

var brass = []string{
	"Trumpet",
	"Cornet",
	"Baritone",
	"Horn",
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
	"Percussion",
	"Melodic percussion",
}

var stringInstruments = []string{
	"Violin",
	"Viola",
	"Cello",
	"Contrabass",
}

var reeds = []string{
	"Saxophone",
	"Clarinet",
	"Oboe",
	"Basoon",
	"Flute",
}

var conductor = []string{
	"Conductor",
}

func allInstruments() []string {
	var allInstruments []string
	allInstruments = append(allInstruments, reeds...)
	allInstruments = append(allInstruments, brass...)
	allInstruments = append(allInstruments, choir...)
	allInstruments = append(allInstruments, stringInstruments...)
	allInstruments = append(allInstruments, percussion...)
	allInstruments = append(allInstruments, conductor...)
	return allInstruments
}

func Instruments(token string) []string {
	return pkg.FilterList(allInstruments(), token)
}
