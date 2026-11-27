// Package project trzyma testy całego repozytorium, a nie jednego pakietu: wzorce API, pokrycie wymagań, katalog
// błędów, importy biblioteki, wrogie dane, wzorce strumieniowe, powtarzalność. Pakiet ma tylko pliki testów; ścieżki
// w testach są liczone od katalogu głównego repozytorium, do którego TestMain przechodzi.
package project

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
