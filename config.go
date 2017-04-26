//
// config.go
// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.

package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"time"
)

type (
	// Configuration keep application configuration
	Configuration struct {
		DBFile    string `yaml:"dbfile"`
		Retention string `yaml:"retention"`
		Debug     bool   `yaml:"debug"`

		RetentionParsed *time.Duration `yaml:"-"`
	}
)

func (c *Configuration) validate() error {
	if c.DBFile == "" {
		c.DBFile = "eventdb.boltdb"
	}
	return nil
}

// LoadConfiguration from `filename`
func LoadConfiguration(filename string) (*Configuration, error) {
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

	if c.Retention != "" {
		r, err := time.ParseDuration(c.Retention)
		if err != nil {
			return nil, fmt.Errorf("parse retention time error: %s", err.Error())
		}
		c.RetentionParsed = &r
	}

	return c, nil
}
