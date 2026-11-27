//go:build race

package goxmlgo

// raceEnabled: testy biegną z detektorem wyścigów, który spowalnia kod kilkanaście razy; granice czasu nie obowiązują.
const raceEnabled = true
