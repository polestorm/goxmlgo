//go:build race

package project

// raceEnabled: testy biegną z detektorem wyścigów, który spowalnia kod kilkanaście razy; granice czasu nie obowiązują.
const raceEnabled = true
