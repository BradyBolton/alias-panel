// Package parser parses aliases either found in $HOME or in files defined in
// $ALIASFILES as a map of Section instances.
package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Alias struct {
	Name string
	Cmd  string
	Desc string
}

type Section struct {
	Label   string
	Aliases map[string]Alias
}

// parseAlias returns Alias parsed from string s (or return an empty struct).
func parseAlias(s string) (Alias, error) {
	var a Alias
	cp := `alias (?P<name>[_a-zA-Z0-9]+)=['"](?P<command>.+)['"][^#]*#(?P<comment>.+)$`
	ap := `alias (?P<name>[_a-zA-Z0-9]+)=['"](?P<command>.+)['"]$`
	ra := regexp.MustCompile(ap)
	rc := regexp.MustCompile(cp)

	// Make two passes to capture (optional) comment since Golang has no
	// conditional regexp (e.g. like JS)
	m := rc.FindStringSubmatch(s)
	if m != nil {
		n := m[1]                    // name
		c := m[2]                    // command
		d := strings.TrimSpace(m[3]) // description/comment
		log.Debugf("parseAlias: %v-%v-%v", n, c, d)
		a = Alias{
			Name: n,
			Cmd:  c,
			Desc: d}
	} else {
		m = ra.FindStringSubmatch(s)
		if m != nil {
			n := m[1] // name
			c := m[2] // command
			log.Debugf("parseAlias: %v-%v", n, c)
			a = Alias{
				Name: n,
				Cmd:  c,
				Desc: ""}
		} else {
			log.Debugf("parseAlias: Skipping %v", s)
			return a, nil
		}
	}
	return a, nil
}

// addAlias processes a line, creating and adding a new Alias to a Section
// if possible.
func addAlias(s Section, line string) Section {
	a, err := parseAlias(line)
	if err != nil {
		log.Error(err)
	} else if a.Name == "" {
		return s
	} else {
		s.Aliases[a.Name] = a
	}

	return s
}

// parseFile produces a map of Sections found in file at path fp.
func parseFile(fp string) map[string]Section {
	sm := make(map[string]Section)
	var cs Section

	sp := `#\s*SECTION:\s*(?P<label>[a-zA-Z0-9 ]+[^\s])`
	rs := regexp.MustCompile(sp)

	f, err := os.Open(fp)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		l := s.Text()

		// Add new detected sections
		if m := rs.FindStringSubmatch(l); m != nil {
			l = m[1]
			if es, ok := sm[l]; ok {
				log.Infof("parseFile: Found existing section <%v>", l)
				cs = es
			} else {
				log.Infof("parseFile: Found new section <%v>", l)
				cs = Section{
					Label:   l,
					Aliases: make(map[string]Alias),
				}
				sm[l] = cs
			}
		} else {
			// If current section is unset before aliases were found
			// then use the default Section (for "orphan aliases")
			if cs.Label == "" {
				cs = Section{
					Label:   "Aliases",
					Aliases: make(map[string]Alias),
				}
				sm["Aliases"] = cs
			}
			cs = addAlias(cs, l)
		}
	}

	// Abort if any Scanner error was detected
	if err := s.Err(); err != nil {
		log.Fatal(err)
	}

	return sm
}

// mergeSections returns the union of two Sections.
func mergeSections(a, b Section) Section {
	for bk, ba := range b.Aliases {
		// Section b's alias overwrites the existing alias in Section a
		if _, ok := a.Aliases[bk]; ok {
			log.Infof("Redefined alias found: %v", bk)
		}
		a.Aliases[bk] = ba
	}
	return a
}

// mergeSectionMaps returns a new map of merged Sections from maps a and b.
func mergeSectionMaps(a, b map[string]Section) map[string]Section {
	for bk, bs := range b {
		// Merge if exists, otherwise append new entry
		if as, ok := a[bk]; ok {
			a[bk] = mergeSections(as, bs)
		} else {
			a[bk] = bs
		}
	}
	return a
}

// parseFiles parses files, returning a map of discovered Sections. Aliases in
// identically named Sections discovered across different files are merged.
func parseFiles(files []string) map[string]Section {
	sm := make(map[string]Section)
	for _, f := range files {
		sm = mergeSectionMaps(sm, parseFile(f))
	}

	// Remove empty sections
	for sl, s := range sm {
		if len(s.Aliases) == 0 {
			log.Infof("Deleting empty section %v", s.Label)
			delete(sm, sl)
		}
	}

	return sm
}

// findFiles returns a slice of paths for files containing aliases to parse.
// If $ALIASFILES' is not specified in the local environment, *_aliases' files
// found in $HOME will be used.
func findFiles() []string {
	fs := make([]string, 0)

	// $ALIASFILES takes precedence over *_aliases files in $HOME
	if ev := os.Getenv("ALIASFILES"); ev != "" {
		fs = append(fs, strings.Split(ev, ":")...)
	} else {
		if h := os.Getenv("HOME"); h != "" {
			hfs, err := os.ReadDir(h)
			if err != nil {
				log.Fatal(err)
			}

			p := `.*_aliases$`
			r := regexp.MustCompile(p)

			for _, f := range hfs {
				fp := filepath.Join(h, f.Name())
				m := r.FindStringSubmatch(fp)
				if m != nil {
					fs = append(fs, fp)
				}
			}
		} else {
			log.Fatal("$HOME not set!")
		}
	}

	return fs
}

// ParseAll parses a map of Sections found in all possible files. Either
// '*_aliases' files found in $HOME or files specified in $ALIASFILES.
func ParseAll() map[string]Section {
	fs := findFiles()
	sm := parseFiles(fs)
	return sm
}
