// Create a man page from a Go package's source file's documentation.
//
//Mango generates man files from the source of a Go package.
//If the package is main, it creates a section 1 man page.
//Otherwise, it creates a section 3 man page.
//
//Mango is a tool similar to godoc(1) and uses the same conventions and  
//information so that documentation will look the same in both.
//Further, Mango uses no special markup.
//The text is formatted by simple rules, described in FORMATTING below.
//The input is the comments and AST of the Go package.
//The output is raw troff markup (compatiable with nroff and groff) dumped 
//to stdout for pipelining, see EXAMPLES.
//
//For section 1 man pages, Mango bases the OPTIONS section on the use of flag(3) 
//and a special comment. It takes
//	var file = flag.String("source", "", "Set source file")
//	var toggle = flag.Bool("t", false, "Toggle mode")
//	var banner = flag.Bool("title", "Hello", "Set banner")
//and produces:
//	-source file
//	-toggle
//	-banner = Hello
//Extraction of this information will not work if the flag package is renamed 
//on import and it can only extract flags defined with flag.Type and not those 
//defined with flag.TypeVar in the init() function.
//Additional information may be given with a comment like
//	//Usage: %name %flags [optional-arg] required-arg
//The "Usage:" part is mandatory.
//%name and %flags (or %flag) exist only to make the intent clear to a 
//reader unfamiliar with mango(1).
//Both may be used together, each may be used separately, and neither are 
//required.
//
//EXAMPLES
//
//Format package in current directory with nroff:
//	mango | nroff -man > name.section
//
//Format package in current directory as a ps file with groff:
//	mango | groff -man > name.section.ps
//
//Format package in current directory as a pdf:
//	mango | groff -man | ps2pdf - name.section.pdf
//
//Format ogle(3), and ogle(1), which exist in the same directory:
//	mango $GOROOT/src/pkg/exp/ogle | nroff -man > ogle.3
//	mango -package main $GOROOT/src/pkg/exp/ogle | nroff -man > ogle.1
//
//Format your package in a makefile:
//	mango -import $TARG $GOFILES | nroff -man > name.section
//
//FORMATTING
//
//An all caps word on a line by itself, followed and preceded by a blank like 
//denotes a new section.
//
//Two consecutive newlines start a new paragraph.
//
//Indented paragraphs, with one tab or four spaces, maintain their relative 
//indentation (a uniform, initial indent is ignored) and no additional
//formatting is applied
//
//Words beginning with a hyphen are assumed to be command line switches, and 
//they, including the hypen, are bolded.
//Words ending with (.) where . is a valid (even if obscure) man page section 
//are formatted with the word but not the (.) in bold.
//In addition, each word is added to the SEE ALSO section.
//
//HEURISTICS
//
//If no directory or files are specified, Mango uses the current working 
//directory.
//If there are multiple packages in the input directory, any package named 
//'documentation' is used as the package level documentation.
//If there is no 'documentation' package or there are still multiple packages 
//after taking 'documentation' out of play, Mango will use the package with the
//same name as the input directory.
//If this is incorrect, or you wish to generate man pages for multiple packages 
//in one directory you can select them individually with the -package flag.
//
//The name of the man page is the name of the package for section 3 man pages 
//and the name of the file that contains func main() for section 1 man pages.
//This can be overridden with the -name flag, but only for section 1 pages.
//
//For man 3 pages, the import path defaults to the name of the package.
//It can be overridden with the -import flag and the
//
//If the -version flag is not used, Mango searches the AST for a const or var 
//declaration named Version.
//Failing that, it uses today's date as the version.
//
//If the -manual flag is not used, it defaults to "User Commands" for man 1 
//pages and to "Go Packages" for man 3 pages, respectively.
//
//Sections in the comments, or specified by the -include or -sections flags, 
//are in the same order as they appear in the comments, with the exception that 
//DIAGNOSTICS, ENVIRONMENT, and FILES section appear first, and in that order; 
//and that HISTORY appears after the SEE ALSO section.
//
//In man 1 pages, the user-defined sections appear between the OPTIONS and 
//BUGS sections. In man 3 pages, they appear after the DESCRIPTION section.
package main

import (
	"flag"
	"os"
	"path"
	"fmt"
	"strings"
	"container/vector"
	"io/ioutil"
	"go/parser"
	"go/ast"
	"go/doc"
)

var (
	help        = flag.Bool("help", false, "Display help")
	import_path = flag.String("import", "",
		"Specify import path")
	name = flag.String("name", "",
		"Specify name of man page")
	version = flag.String("version", "",
		"Specify version")
	manual = flag.String("manual", "",
		"Specify the manual: see man-pages(7)")
	package_name = flag.String("package", "",
		"Select package to use if there are multiple packages in a directory")
	Sections = flag.String("section", "",
		`Generate sections from a comma-seperated list of filenames. Each section will
be named after the file name that contains it (_ will be replaced by a space).
The contents of each file will be formatted by the same rules as if they were
extracted from comments. To include preformatted sections see -include.
You cannot override SYNOPSIS, OPTIONS, BUGS or SEE ALSO.`)

	Includes = flag.String("include", "",
		`Generate sections from a comma-seperated list of filenames. Each section will
be named after the file name that contains it (_ will be replaced by a space).
The contents of each file will be included as-is. To let mango do the formatting
use -section.`)
)


func stderr(s interface{}) {
	fmt.Fprintln(os.Stderr, s)
}

func fatal(msg interface{}) {
	stderr(msg)
	os.Exit(2)
}

func invalid_flag(s, nm string, flag *string) {
	if *flag != "" {
		fatal("The " + nm + " flag does not apply to section " + s + " pages.")
	}
}

func lspkgs(dir string, pkgs map[string]*ast.Package) {
	stderr(dir + " contains the following packages:")
	for k := range pkgs {
		stderr("\t" + k)
	}
	stderr("Don't know how to handle a directory with multiple packages")
	fatal("Specify one of the above with -package")
}

func usage(err interface{}) {
	if err != nil {
		stderr(err)
	}
	stderr("mango [flags] [package-directory|package-files]\nflags:")
	flag.PrintDefaults()
	os.Exit(1)
}

var suff = strings.HasSuffix
var pref = strings.HasPrefix

func filter(fi *os.FileInfo) bool {
	notdir := !fi.IsDirectory()
	n := fi.Name
	gofile := suff(n, ".go") && !suff(n, "_test.go")
	return notdir && gofile
}

func clean(pwd, p string) string {
	if !path.IsAbs(p) {
		p = path.Clean(path.Join(pwd, p))
	}
	return path.Clean(p)
}

type pair struct {
	key   string
	value []byte
}

func csv_files(in string, disallow bool) (out []*pair) {
	for _, fname := range strings.Split(in, ",", -1) {
		sname := strings.TrimSpace(fname)
		sname = strings.ToUpper(fname)
		sname = strings.Replace(sname, "_", " ", -1)
		if disallow {
			switch sname {
			case "SEE ALSO", "SYNOPSIS", "OPTIONS", "BUGS":
				fatal("Cannot override SEE ALSO, BUGS, or OPTIONS")
			}
		}
		bytes, err := ioutil.ReadFile(fname)
		if err != nil {
			fatal(err)
		}
		if sname == "DESCRIPTION" {
			sname = ""
		}
		out = append(out, &pair{sname, bytes})
	}
	return out
}

//Usage: %name %flags [package-directory|package-files]
func main() {
	flag.Parse()

	if *help {
		usage(nil)
	}

	pwd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

	var pkg *ast.Package
	var pkgs map[string]*ast.Package

	//Select and parse files
	dir := pwd
	var files []string
	if flag.NArg() == 1 {
		//one arg we assume is a directory name
		dir = clean(pwd, flag.Arg(0))
		//but it could be a package contained in one .go file
		if suff(dir, ".go") {
			files = []string{dir}
		}
	} else if flag.NArg() > 1 {
		//more than one args we assume is multiple files so we can do
		//mango -import $TARG $GOFILES
		//in make files
		files = flag.Args()
		for i, file := range files {
			files[i] = clean(pwd, file)
		}
	}
	//parse package(s)
	if len(files) > 0 {
		pkgs, err = parser.ParseFiles(files, parser.ParseComments)
	} else {
		pkgs, err = parser.ParseDir(dir, filter, parser.ParseComments)
	}
	if err != nil {
		fatal("Could not parse " + dir + "\n" + err.String())
	}

	//Check for a documentation package
	var xdoc string
	if d, ok := pkgs["documentation"]; ok {
		xdoc = doc.NewPackageDoc(d, "").Doc
		pkgs["documentation"] = nil, false
	}

	//Select package
	switch len(pkgs) {
	case 0:
		fatal("No packages found at " + dir)
	case 1:
		// what we want
	default:
		if *package_name != "" {
			var ok bool
			pkg, ok = pkgs[*package_name]
			if !ok {
				lspkgs(dir, pkgs)
			}
		} else {
			name := path.Base(dir)
			if p, ok := pkgs[name]; ok {
				pkg = p
			} else {
				lspkgs(dir, pkgs)
			}
		}
	}
	if pkg == nil {
		//hack because we don't know or care what the package is
		for _, v := range pkgs {
			pkg = v
		}
	}

	var overd []*section
	if *Sections != "" {
		for _, pair := range csv_files(*Sections, true) {
			overd = append(overd,
				&section{pair.key, unstring(pair.value)})
		}
	}
	if *Includes != "" {
		for _, pair := range csv_files(*Includes, false) {
			overd = append(overd,
				&section{pair.key, &vector.Vector{pair.value}})
		}
	}

	//Build and dump docs
	docs := doc.NewPackageDoc(pkg, "")
	//hack around there being a documentation package, part 2
	if xdoc != "" {
		docs.Doc = xdoc
	}
	m := NewManPage(pkg, docs, overd)

	if pkg.Name == "main" {
		invalid_flag("1", "import", import_path)
		doCommand(m)
	} else {
		invalid_flag("3", "name", name)
		doPackage(m)
	}

	m.nl()
	os.Stdout.Write(m.Bytes())
}
