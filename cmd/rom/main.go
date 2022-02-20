package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bodgit/rom"
	"github.com/bodgit/rom/dat"
	"github.com/bodgit/rom/synchronizer"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var stringToChecksum = map[string]rom.Checksum{
	"crc32": rom.CRC32,
	"md5":   rom.MD5,
	"sha1":  rom.SHA1,
}

type enumValue struct {
	Enum     []string
	Default  string
	selected string
}

func (e *enumValue) Set(value string) error {
	for _, enum := range e.Enum {
		if enum == value {
			e.selected = value
			return nil
		}
	}

	return fmt.Errorf("allowed values are %s", strings.Join(e.Enum, ", "))
}

func (e *enumValue) String() string {
	if e.selected == "" {
		return e.Default
	}
	return e.selected
}

func init() {
	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "print the version",
	}
}

func sync(c *cli.Context) error {
	if c.NArg() < 1 {
		cli.ShowCommandHelpAndExit(c, c.Command.FullName(), 1)
	}

	logger := log.New(ioutil.Discard, "", 0)
	if c.Bool("verbose") {
		logger.SetOutput(os.Stderr)
	}

	s, err := synchronizer.NewSynchronizer(synchronizer.Logger(logger), synchronizer.Workers(c.Int("workers")), synchronizer.DryRun(c.Bool("dry-run")), synchronizer.Checksum(stringToChecksum[c.Generic("algorithm").(*enumValue).String()]))
	if err != nil {
		log.Fatal(err)
	}

	if c.Path("mia") != "" {
		f, err := os.Open(c.Path("mia"))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		if err = s.SetMissing(f); err != nil {
			log.Fatal(err)
		}
	}

	start := time.Now()
	db, err := s.Scan(c.Args().Slice()...)
	if err != nil {
		log.Fatal(err)
	}
	elapsed := time.Since(start)

	logger.Println("Read", s.Rx(), "bytes in", elapsed)

	s.Reset()

	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	datfile := new(dat.File)
	if err = xml.Unmarshal(b, datfile); err != nil {
		log.Fatal(err)
	}

	start = time.Now()
	if err = s.Update(c.Args().First(), datfile, db); err != nil {
		log.Fatal(err)
	}
	elapsed = time.Since(start)

	logger.Println("Read", s.Rx(), "bytes and wrote", s.Tx(), "bytes in", elapsed)

	s.Delete(c.Args().First(), datfile)

	if b, err = xml.MarshalIndent(datfile, "", "\t"); err != nil {
		log.Fatal(err)
	}

	if len(b) > 0 {
		// Need to add a final newline if there is some XML
		if _, err = os.Stdout.Write(append(b, []byte("\n")...)); err != nil {
			log.Fatal(err)
		}
	}

	return nil
}

func info(c *cli.Context) error {
	if c.NArg() < 1 {
		cli.ShowCommandHelpAndExit(c, c.Command.FullName(), 1)
	}

	for i, r := range c.Args().Slice() {
		reader, err := rom.NewReader(r)
		if err != nil {
			log.Fatal(err)
		}

		if i > 0 {
			fmt.Println()
		}

		fmt.Println(r)
		fmt.Println()

		table := tablewriter.NewWriter(os.Stdout)
		table.SetBorder(false)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetAutoWrapText(false)

		table.SetHeader([]string{"ROM", "Size", "Header", "CRC32", "MD5", "SHA1"})

		files := reader.Files()
		sort.Strings(files)

		for _, f := range files {
			size, header, err := reader.Size(f)
			if err != nil {
				log.Fatal(err)
			}

			c, err := reader.Checksum(f, rom.CRC32)
			if err != nil {
				log.Fatal(err)
			}

			m, err := reader.Checksum(f, rom.MD5)
			if err != nil {
				log.Fatal(err)
			}

			s, err := reader.Checksum(f, rom.SHA1)
			if err != nil {
				log.Fatal(err)
			}

			table.Append([]string{f, strconv.FormatUint(size-header, 10), strconv.FormatUint(header, 10), fmt.Sprintf("%x", c), fmt.Sprintf("%x", m), fmt.Sprintf("%x", s)})
		}

		table.Render()

		reader.Close()
	}

	return nil
}

func main() {
	app := cli.NewApp()

	app.Name = "rom"
	app.Usage = "ROM management utility"
	app.Version = fmt.Sprintf("%s, commit %s, built at %s", version, commit, date)

	checksums := make([]string, 0, len(stringToChecksum))
	for k := range stringToChecksum {
		checksums = append(checksums, k)
	}
	sort.Sort(sort.StringSlice(checksums))

	app.Commands = []*cli.Command{
		{
			Name:        "info",
			Usage:       "ROM information",
			Description: "",
			Action:      info,
			ArgsUsage:   "",
		},
		{
			Name:        "sync",
			Usage:       "Synchronise ROMs",
			Description: "Build a directory of Torrentzipped ROMs",
			Action:      sync,
			ArgsUsage:   "TARGET [SOURCE...]",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "dry-run",
					Aliases: []string{"n"},
					Usage:   "don't actually do anything",
				},
				&cli.IntFlag{
					Name:    "workers",
					Aliases: []string{"w"},
					Usage:   "number of workers",
					Value:   runtime.NumCPU(),
				},
				&cli.BoolFlag{
					Name:    "verbose",
					Aliases: []string{"v"},
					Usage:   "increase verbosity",
				},
				&cli.GenericFlag{
					Name:    "algorithm",
					Aliases: []string{"a"},
					Value: &enumValue{
						Enum:    checksums,
						Default: "crc32",
					},
					Usage: "checksum algorithm to use. (" + strings.Join(checksums, ", ") + ")",
				},
				&cli.PathFlag{
					Name:    "mia",
					Aliases: []string{"m"},
					Usage:   "path to file containing list of games to ignore",
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
