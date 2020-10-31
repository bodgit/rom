package dat

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

func ExampleUnmarshal() {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "NEC - PC Engine SuperGrafx (20191008-080644).dat"))
	if err != nil {
		panic(err)
	}

	f := new(File)
	if err := xml.Unmarshal(b, f); err != nil {
		panic(err)
	}

	for _, g := range f.Game {
		fmt.Println(g.Name)
	}

	// Output: 1941 - Counter Attack (Japan)
	// Aldynes - The Misson Code for Rage Crisis (Japan)
	// Battle Ace (Japan)
	// Daimakaimura (Japan)
	// Madou King Granzort (Japan)
}

func ExampleMarshal() {
	f := File{
		Header: Header{
			Name:        "test",
			Description: "description",
			Version:     "1",
			Date:        "1/1/1970",
			Author:      "author",
			Homepage:    "homepage",
			URL:         "http://example.com",
		},
		Game: []Game{
			{
				Name:        "test",
				Category:    "category",
				Description: "description",
				ROM: []ROM{
					{
						Name:  "test.bin",
						Size:  123,
						CRC32: "123",
						MD5:   "456",
						SHA1:  "789",
					},
				},
			},
		},
	}

	f.Game[0].ROM[0].Matched()
	f.Reset()

	b, err := xml.MarshalIndent(&f, "", "\t")
	if err != nil {
		panic(err)
	}

	fmt.Println(string(b))

	// Output: <datafile>
	//	<header>
	//		<name>test</name>
	//		<description>description</description>
	//		<version>1</version>
	//		<date>1/1/1970</date>
	//		<author>author</author>
	//		<homepage>homepage</homepage>
	//		<url>http://example.com</url>
	//	</header>
	//	<game name="test">
	//		<category>category</category>
	//		<description>description</description>
	//		<rom name="test.bin" size="123" crc="123" md5="456" sha1="789"></rom>
	//	</game>
	//</datafile>
}
