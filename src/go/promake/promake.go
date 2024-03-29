package main

import (
	"fmt"
	"os"
	"io/ioutil"
	"bytes"
	"regexp"
	"flag"
	"path"
	"exec"
	"syscall"
)

var (
	proFileName  *string = flag.String("gopro", "", "make go project")
	goFileName   *string = flag.String("gofiles", "", "make go sources")
	goTargetName *string = flag.String("o", "", "file specify output file")
	printDep     *bool   = flag.Bool("dep", false, "print packages depends ")
	showVer      *bool   = flag.Bool("ver", true, "print version ")
	buildLib     *bool   = flag.Bool("lib", false, "build packages as librarys outside main")
	goroot		 *string = flag.String("goroot",defGoroot(),"default go root")	
)

type Gopro struct {
	Name   string
	Values map[string][]string
	array  *PackageArray
}

func makePro(name string) (pro *Gopro, err os.Error) {
	buf, e := ioutil.ReadFile(name)
	if e != nil {
		err = e
		return
	}
	pro = new(Gopro)
	pro.Name = name
	pro.Values = make(map[string][]string)

	pre, e := regexp.Compile("\\\\[\t| ]*[\r|\n]+[\t| ]*")//("\\\\[^a-z|A-Z|0-9|_|\r|\n]*[\r|\n]+[^a-z|A-Z|0-9|_|\r|\n]*")
	if e != nil {
		err = e
		return
	}
	all := pre.ReplaceAll(buf[:], []byte(" "))
	all = bytes.Replace(all, []byte("\r\n"), []byte("\n"), -1)
	lines := bytes.Split(all, []byte("\n"), -1)

	for _, line := range lines {
		offset := 2
		line = bytes.Replace(line, []byte("\t"), []byte(" "), -1)
		if len(line) >= 1 && line[0] == '#' {
			continue
		}
		find := bytes.Index(line, []byte("+="))
		if find == -1 {
			offset = 1
			find = bytes.Index(line, []byte("="))
		}
		if find != -1 {
			k := bytes.TrimSpace(line[0:find])
			v := bytes.SplitAfter(line[find+offset:], []byte(" "), -1)
			var vall []string
			if offset == 2 {
				vall = pro.Values[string(k)]
			}
			for _, nv := range v {
				nv2 := bytes.TrimSpace(nv)
				if len(nv2) != 0 {
					vall = append(vall, string(nv2))
				}
			}
			pro.Values[string(k)] = vall
		}
	}
	return
}

func (file *Gopro) Gofiles() []string {
	return file.Values["GOFILES"]
}

func (file *Gopro) AllPackage() (paks []string) {
	for i := 0; i < len(file.array.Data); i++ {
		paks = append(paks, file.array.Data[i].pakname)
	}
	return
}

func (file *Gopro) PackageFilesString(pakname string) []string {
	return file.Values[pakname]
}

func (file *Gopro) PackageFiles(pakname string) []string {
	return file.array.index(pakname).files
}

func (file *Gopro) TargetName() string {
	v := file.Values["TARGET"]
	if len(v) >= 1 && len(v[0]) > 0 {
		name := v[0]
		ext := path.Ext(name)
		return name[0 : len(name)-len(ext)]
	}
	if len(file.Name) == 0 {
		return "main"
	}
	ext := path.Ext(file.Name)
	name := path.Base(file.Name)
	return name[0 : len(name)-len(ext)]
}

func (file *Gopro) DestDir() (dir string) {
	v := file.Values["DESTDIR"]
	if len(v) >= 1 {
		dir = string(v[0])
	}
	return
}

func (file *Gopro) ProjectDir() (dir string) {
	dir, _ = path.Split(file.Name)
	return
}

func build(gcfile string, opts []string, proFileName string, files []string, envv []string, dir string) (status syscall.WaitStatus, err os.Error) {
	arg := []string{gcfile, "-o", proFileName}
	for _,v := range opts {
		arg = append(arg,v)
	}
	for _, v := range files {
		arg = append(arg, string(v))
	}
	fmt.Println("\t", arg)
	var cmd *exec.Cmd
	cmd, err = exec.Run(gcfile, arg[:], envv[:], dir, 0, 1, 2)
	if err != nil {
		fmt.Printf("Error, %s", err)
		return
	}
	defer cmd.Close()
	var wait *os.Waitmsg
	wait, err = cmd.Wait(0)
	if err != nil {
		fmt.Printf("Error, %s", err)
		return
	}
	status = wait.WaitStatus
	return
}

func link(glfile string, opts []string, target string, ofile string, envv []string, dir string) (status syscall.WaitStatus, err os.Error) {
	arg := []string{glfile, "-o", target}
	for _,v := range opts {
		arg = append(arg,v)
	}	
	arg = append(arg,ofile)
	fmt.Println("\t", arg)
	var cmd *exec.Cmd
	cmd, err = exec.Run(glfile, arg[:], envv[:], dir, 0, 1, 2)
	if err != nil {
		fmt.Printf("Error, %s", err)
		return
	}
	defer cmd.Close()
	var wait *os.Waitmsg
	wait, err = cmd.Wait(0)
	if err != nil {
		fmt.Printf("Error, %s", err)
		return
	}
	status = wait.WaitStatus
	return
}

func pack(pkfile string, target string, ofile string, envv []string, dir string) (status syscall.WaitStatus, err os.Error) {
	arg := []string{pkfile, "grc", target, ofile}
	fmt.Println("\t", arg)
	var cmd *exec.Cmd
	cmd, err = exec.Run(pkfile, arg[:], envv[:], dir, 0, 1, 2)
	if err != nil {
		fmt.Printf("Error, %s", err)
		return
	}
	defer cmd.Close()
	var wait *os.Waitmsg
	wait, err = cmd.Wait(0)
	if err != nil {
		fmt.Printf("Error, %s", err)
		return
	}
	status = wait.WaitStatus
	return
}

func (file *Gopro) IsEmpty() bool {
	return len(file.Values) == 0
}

func (file *Gopro) MakeTarget(gobin *GoBin) (status syscall.WaitStatus, err os.Error) {
	all := file.AllPackage()
	for _, v := range all {
		if v == "documentation" {
			continue
		}			
		fmt.Printf("build package %s:\n", v)
		target := v
		ofile := target + gobin.objext
		if v == "main" {
			target = file.TargetName()
			ofile = target + "_go_" + gobin.objext
		} 
		status, err = build(gobin.compiler, file.Values["GCOPT"], ofile, file.PackageFiles(v), os.Environ(), file.ProjectDir())
		if err != nil || status.ExitStatus() != 0 {
			return
		}

		dest := file.DestDir()
		if len(dest) > 0 {
			//dest = path.Join(file.ProjectDir(), dest)
			os.MkdirAll(dest, 0777)
			target = path.Join(dest, target)
		}
		if string(v) == "main" {
			target = target + gobin.exeext
			status, err = link(gobin.link, file.Values["GLOPT"],target, ofile, os.Environ(), file.ProjectDir())
			if err != nil || status.ExitStatus() != 0 {
				return
			}
			fmt.Printf("link target : %s\n", target)
		} else if *buildLib {
			target = target + gobin.pakext
			status, err = pack(gobin.pack, target, ofile, os.Environ(), file.ProjectDir())
			if err != nil || status.ExitStatus() != 0 {
				return
			}
			fmt.Printf("pack library : %s\n", target)
		}
	}
	return
}

var Usage = func() {
	_, name := path.Split(os.Args[0])
	fmt.Fprintf(os.Stderr, "usage: %s -gopro   example.pro\n", name)
	fmt.Fprintf(os.Stderr, "       %s -gofiles \"go1.go go2.go\"\n", name)
	flag.PrintDefaults()
}

func exitln(err os.Error) {
	fmt.Println(err)
	os.Exit(1)
}

func main() {
	flag.Parse()
	if *showVer == true {
		fmt.Println("GoproMake 0.2.1: go files auto build tools. make by visualfc@gmail.com.")
	}

	gobin, err := newGoBin(*goroot)
	if err != nil {
		exitln(err)
	}

	var pro *Gopro

	if len(*proFileName) > 0 {
		pro, err = makePro(*proFileName)
		if err != nil {
			exitln(err)
		}
	} else if len(*goFileName) > 0 {
		var input []byte = []byte(*goFileName)
		all := bytes.SplitAfter(input, []byte(" "), -1)
		pro = new(Gopro)
		pro.Values = make(map[string][]string)

		for _, v := range all {
			pro.Values["GOFILES"] = append(pro.Values["GOFILES"], string(v))
		}
	}
	if pro == nil || err != nil {
		Usage()
		os.Exit(1)
	}

	if len(*goTargetName) > 0 {
		pro.Values["TARGET"] = []string{*goTargetName}
	}
	fmt.Println("GoproMake parser files...")

	files := pro.Gofiles()
	pro.array = ParserFiles(files)

	if printDep != nil && *printDep == true {
		fmt.Printf("AllPackage:\n%s\n", pro.array)
	}

	if pro.array.HasMain == false {
		*buildLib = true
	}

	status, err := pro.MakeTarget(gobin)
	if err != nil {
		exitln(err)
	} else if status.ExitStatus() != 0 {
		exitln(os.NewError("Error"))
	}
	os.Exit(0)
}
