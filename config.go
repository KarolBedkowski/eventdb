//
// config.go
// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.

package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type (
	// Configuration keep application configuration
	Configuration struct {
		DBFile string
	}
)

func (c *Configuration) validate() error {
	if c.DBFile == "" {
		c.DBFile = "eventdb.boltdb"
	}
	return nil
}

func loadConfiguration(filename string) (*Configuration, error) {
	c := &Configuration{}
	b, err := ioutil.ReadFile(filename)

	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(b, c); err != nil {
		return nil, err
	}

	if err = c.validate(); err != nil {
		return nil, err
	}

	return c, nil
}
