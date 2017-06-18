//
// config.go
// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.

package main

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"time"
)

type (
	// PromWebHookConf configure api for Prometheus Alertmanager
	PromWebHookConf struct {
		MappedLabels []string `yaml:"mapped_labels"`

		Bucket string `yaml:"bucket"`
	}

	// AnnotationsConf configure api for Grafana annotations
	AnnotationsConf struct {
		ReturnedCols []string `yaml:"returned_cols"`
	}

	// Configuration keep application configuration
	Configuration struct {
		DBFile    string `yaml:"dbfile"`
		Retention string `yaml:"retention"`
		Debug     bool   `yaml:"debug"`

		PromWebHookConf PromWebHookConf `yaml:"promwebhool_conf"`
		AnnotationsConf AnnotationsConf `yaml:"annotations_conf"`

		DefaultBucket string `yaml:"default_bucket"`

		RetentionParsed *time.Duration `yaml:"-"`
	}
)

func (c *Configuration) validate() error {
	if c.DBFile == "" {
		c.DBFile = "eventdb.boltdb"
	}
	if c.DefaultBucket == "" {
		c.DefaultBucket = "__default__"
	}
	if c.PromWebHookConf.Bucket == "" {
		c.PromWebHookConf.Bucket = c.DefaultBucket
	}
	return nil
}

// LoadConfiguration from `filename`
func LoadConfiguration(filename string) (*Configuration, error) {
	c := &Configuration{}
	b, err := ioutil.ReadFile(filename)

	if err != nil {
		return nil, errors.Wrapf(err, "read file %v failed", filename)
	}

	if err = yaml.Unmarshal(b, c); err != nil {
		return nil, errors.Wrap(err, "unmarshal error")
	}

	if err = c.validate(); err != nil {
		return nil, errors.Wrap(err, "validate error")
	}

	if c.Retention != "" {
		r, err := time.ParseDuration(c.Retention)
		if err != nil {
			return nil, errors.Wrap(err, "parse retention time error")
		}
		c.RetentionParsed = &r
	}

	return c, nil
}
