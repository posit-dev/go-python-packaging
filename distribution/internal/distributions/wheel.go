// SPDX-License-Identifier: Apache-2.0 OR MIT
//
// Portions of this file are ported from pypa/twine
// (https://github.com/pypa/twine), Copyright (c) 2013 Donald Stufft and
// individual contributors and Copyright (c) 2015 Ian Cordasco, specifically
// its Wheel class: the WHEEL_FILE_RE pattern (wheelFileRe below) and the
// read()/python_version logic that picks the shortest dist-info/METADATA
// path containing a "Metadata-Version" line, used under the Apache License,
// Version 2.0. See NOTICE for full license and copyright detail.
// Changed: translated from Python to Go; archive reading goes through this
// package's own archiver.ArchiveReader abstraction instead of Python's
// zipfile.

package distributions

import (
	"bytes"
	"fmt"
	"github.com/posit-dev/go-python-packaging/distribution/internal/archiver"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var wheelFileRe = regexp.MustCompile(`^(?P<namever>(?P<name>.+?)(-(?P<ver>\d.+?))?)(?:(-(?P<build>\d.*?))?-(?P<pyver>.+?)-(?P<abi>.+?)-(?P<plat>.+?)\.whl|\.dist-info)$`)

type Wheel struct {
	BaseDistribution
	Filename     string `json:"file_name"`
	BaseFilename string `json:"base_filename"`
}

func NewWheel(filename string) (Distribution, error) {
	wheel := &Wheel{}
	wheel.Filename = filename
	wheel.BaseFilename = filepath.Base(filename)
	err := wheel.ExtractMetadata()
	if err != nil {
		return nil, err
	}
	return wheel, nil
}

func (whl *Wheel) MetadataMap() map[string][]string {
	return StructToMap(*whl)
}

func (whl *Wheel) ExtractMetadata() error {
	data, err := whl.read()
	if err != nil {
		return err
	}
	err = whl.Parse(data)
	if err != nil {
		return err
	}
	return nil
}

func (whl *Wheel) read() ([]byte, error) {
	filename := whl.Filename
	fqn, err := filepath.Abs(filepath.Clean(filename))
	if err != nil {
		return nil, fmt.Errorf("error normalizing path: %w", err)
	}

	archiveReader, err := archiver.NewArchiveReader(fqn)
	if err != nil {
		return nil, fmt.Errorf("error getting archive: %w", err)
	}
	defer func(archiveReader archiver.ArchiveReader) {
		err := archiveReader.Close()
		if err != nil {
			log.Printf("error closing reader: %v", err)
		}
	}(archiveReader) // Ensure the archive is closed after reading

	fileNames, err := archiveReader.FileNames()
	if err != nil {
		return nil, err
	}

	var tuples [][]string
	for _, name := range fileNames {
		if strings.Contains(name, "METADATA") {
			tuples = append(tuples, strings.Split(name, "/"))
		}
	}

	sort.Slice(tuples, func(i, j int) bool {
		return len(tuples[i]) < len(tuples[j])
	})

	for _, path := range tuples {
		candidate := strings.Join(path, "/")
		data, err := archiveReader.ReadFile(candidate)
		if err != nil {
			return nil, fmt.Errorf("error reading file %s from archive: %v", candidate, err)
		}
		if bytes.Contains(data, []byte("Metadata-Version")) {
			return data, nil
		}
	}
	return nil, fmt.Errorf("no METADATA in archive: %s", fqn)
}

func (whl *Wheel) GetPythonVersion() string {
	wheelInfo := wheelFileRe.FindStringSubmatch(whl.BaseFilename)
	if wheelInfo == nil {
		return "any"
	}

	for i, name := range wheelFileRe.SubexpNames() {
		if name == "pyver" {
			return wheelInfo[i]
		}
	}

	return "any"
}
