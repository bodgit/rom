/*
Package dat implements parsing of XML dat files as commonly used by ROM/Disc
preservation projects such as http://redump.org and https://no-intro.org. The
DTD supports more elements but currently just the minimal subset used by
these two projects are implemented.

It has the facility to mark a ROM as matched/found and so when the File is
marshalled back to XML, any such ROMs are not included in the output. This
means if all of the ROMs for a particular Game are matched, that Game will not
be included at all in the output and consequently if all ROMs for all Games
are matched, no XML will be output at all for the entire File.

An example:

        import (
                "encoding/xml"
                "os"

                "github.com/bodgit/rom/dat"
        )

        func main() {
                b, err := os.ReadFile(os.Args[1])
                if err != nil {
                        panic(err)
                }

                f := new(dat.File)
                if err := xml.Unmarshal(b, f); err != nil {
                        panic(err)
                }

                // Mark the first ROM of the first Game as matched
                f.Game[0].ROM[0].Matched()

                b, err = xml.MarshalIndent(f, "", "\t")
                if err != nil {
                        panic(err)
                }

                fmt.Println(string(b))
        }
*/
package dat

// BUG(bodgit): Due to how encoding/xml works, <rom> elements are not marshalled as self-closing

import (
	"encoding/xml"
	"strconv"
	"strings"

	"github.com/bodgit/rom"
)

// Header represents the header section in the XML dat file
type Header struct {
	XMLName     xml.Name `xml:"header"`
	Name        string   `xml:"name"`
	Description string   `xml:"description"`
	Version     string   `xml:"version"`
	Date        string   `xml:"date"`
	Author      string   `xml:"author"`
	Homepage    string   `xml:"homepage"`
	URL         string   `xml:"url"`
}

// File represents the whole XML dat file. It consists of one Header followed
// zero or more Games
type File struct {
	XMLName xml.Name `xml:"datafile"`
	Header  Header   `xml:"header"`
	Game    []Game   `xml:"game"`
}

func (f *File) isComplete() bool {
	complete := 0
	for _, g := range f.Game {
		if g.isComplete() {
			complete++
		}
	}
	return complete == len(f.Game)
}

// MarshalXML is required by the xml.Marshaler interface. It encodes the File
// as XML if at least one of its Games has at least one ROM that has not been
// matched
func (f *File) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if f.isComplete() {
		return nil
	}

	// Need to override this
	start = xml.StartElement{Name: xml.Name{Local: "datafile"}}

	if err := e.EncodeToken(start); err != nil {
		return err
	}

	if err := e.EncodeElement(f.Header, xml.StartElement{Name: xml.Name{Local: "header"}}); err != nil {
		return err
	}

	for _, g := range f.Game {
		if g.isComplete() {
			continue
		}
		if err := e.EncodeElement(g, xml.StartElement{Name: xml.Name{Local: "game"}}); err != nil {
			return err
		}
	}

	if err := e.EncodeToken(start.End()); err != nil {
		return err
	}

	return e.Flush()
}

// Reset returns each Game within File f back to its original state
func (f *File) Reset() {
	for i := range f.Game {
		f.Game[i].Reset()
	}
}

// Game represents one game within an XML dat file. It contains zero or more
// ROMs
type Game struct {
	XMLName     xml.Name `xml:"game"`
	Name        string   `xml:"name,attr"`
	Category    string   `xml:"category"`
	Description string   `xml:"description"`
	ROM         []ROM    `xml:"rom"`
}

// Matched marks Game g as found in some external repository. By doing this
// it will not be marshalled back into XML
func (g *Game) Matched() {
	for i := range g.ROM {
		g.ROM[i].Matched()
	}
}

func (g *Game) isComplete() bool {
	complete := 0
	for _, r := range g.ROM {
		if r.isComplete() {
			complete++
		}
	}
	return complete == len(g.ROM)
}

// Reset returns each ROM used by Game g back to its original state
func (g *Game) Reset() {
	for i := range g.ROM {
		g.ROM[i].Reset()
	}
}

// ROM represents one ROM within an XML dat file
type ROM struct {
	XMLName xml.Name `xml:"rom"`
	Name    string   `xml:"name,attr"`
	Size    uint64   `xml:"size,attr"`
	CRC32   string   `xml:"crc,attr"`
	MD5     string   `xml:"md5,attr"`
	SHA1    string   `xml:"sha1,attr"`
	matched bool
}

// Checksum returns the correct checksum value based on the requested
// checksum type
func (r *ROM) Checksum(t rom.Checksum) string {
	var v string
	switch t {
	case rom.CRC32:
		v = strings.ToLower(r.CRC32)
	case rom.MD5:
		v = strings.ToLower(r.MD5)
	case rom.SHA1:
		v = strings.ToLower(r.SHA1)
	}
	return v
}

// MarshalXML is required by the xml.Marshaler interface. It encodes the ROM
// as XML if the ROM has not been matched
func (r *ROM) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if r.isComplete() {
		return nil
	}

	start.Attr = []xml.Attr{
		{
			Name:  xml.Name{Local: "name"},
			Value: r.Name,
		},
		{
			Name:  xml.Name{Local: "size"},
			Value: strconv.FormatUint(r.Size, 10),
		},
		{
			Name:  xml.Name{Local: "crc"},
			Value: r.CRC32,
		},
		{
			Name:  xml.Name{Local: "md5"},
			Value: r.MD5,
		},
		{
			Name:  xml.Name{Local: "sha1"},
			Value: r.SHA1,
		},
	}
	tokens := []xml.Token{start}

	for _, t := range tokens {
		if err := e.EncodeToken(t); err != nil {
			return err
		}
	}

	if err := e.EncodeToken(start.End()); err != nil {
		return err
	}

	return e.Flush()
}

// Matched marks ROM r as found in some external repository. By doing this
// it will not be marshalled back into XML
func (r *ROM) Matched() {
	r.matched = true
}

func (r *ROM) isComplete() bool {
	return r.matched
}

// Reset returns ROM r to its original state such that it will be marshalled
// back into XML
func (r *ROM) Reset() {
	r.matched = false
}
