// Copyright © 2019 Dmitry Mozzherin <dmozzherin@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/gnames/gnparser"
	"github.com/gnames/gnparser/output"
	"github.com/gnames/gnparser/rpc"
	"github.com/gnames/gnparser/web"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gnparser file_or_name",
	Short: "Parses scientific names into their semantic elements.",
	Long: `
Parses scientific names into their semantic elements.

To see version:
gnparser -v

To parse one name in CSV format
gnparser "Homo sapiens Linnaeus 1753" [flags]
or (the same)
gnparser "Homo sapiens Linnaeus 1753" -f csv [flags]

To parse one name using JSON format:
gnparser "Homo sapiens Linnaeus 1753" -f compact [flags]
or
gnparser "Homo sapiens Linnaeus 1753" -f pretty [flags]

To parse many names from a file (one name per line):
gnparser names.txt [flags] > parsed_names.txt

To leave HTML tags and entities intact when parsing (faster)
gnparser names.txt -n > parsed_names.txt

To start gRPC parsing service on port 3355 with a limit
of 10 concurrent jobs per request:
gnparser -j 10 -g 3355

To start web service on port 8080 with 5 concurrent jobs:
gnparser -j 5 -g 8080
 `,

	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		versionFlag(cmd)
		wn := workersNumFlag(cmd)

		nocleanup := skipCleanupFlag(cmd)

		grpcPort := grpcFlag(cmd)
		if grpcPort != 0 {
			fmt.Println("Running gnparser as gRPC service:")
			fmt.Printf("port: %d\n", grpcPort)
			fmt.Printf("Max jobs per request: %d\n\n", wn)
			rpc.Run(grpcPort, wn)
			os.Exit(0)
		}

		webPort := webFlag(cmd)
		if webPort != 0 {
			fmt.Println("Running gnparser as a website and REST server:")
			fmt.Printf("port: %d\n", webPort)
			fmt.Printf("jobs: %d\n\n", wn)
			web.Run(webPort, wn)
			os.Exit(0)
		}
		f := formatFlag(cmd)
		opts := []gnparser.Option{
			gnparser.OptWorkersNum(wn),
			gnparser.OptFormat(f),
			gnparser.OptRemoveHTML(!nocleanup),
		}
		if len(args) == 0 {
			processStdin(cmd, wn, opts)
			os.Exit(0)
		}
		data := getInput(cmd, args)
		parse(data, wn, opts)
	},
}

// Execute adds all child commands to the root command and sets flags
// appropriately. This is called by main.main(). It only needs to happen once to
// the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	gnp := gnparser.NewGNparser()
	rootCmd.PersistentFlags().BoolP("version", "v", false, "shows build version and date, ignores other flags.")

	df := gnp.OutputFormat()
	formats := strings.Join(gnparser.AvailableFormats(), ", ")
	formatHelp := fmt.Sprintf("sets output format. Can be one of:\n %s.", formats)
	rootCmd.Flags().StringP("format", "f", df, formatHelp)

	rootCmd.Flags().IntP("jobs", "j", runtime.NumCPU(),
		"nubmer of threads to run. CPU's threads number is the default.")

	rootCmd.Flags().BoolP("nocleanup", "n", false, "keep HTML entities and tags when parsing.")

	rootCmd.Flags().IntP("grpc_port", "g", 0, "starts gRPC server on the port.")

	rootCmd.Flags().IntP("web_port", "w", 0,
		"starts web site and REST server on the port.")
}

func versionFlag(cmd *cobra.Command) {
	version, err := cmd.Flags().GetBool("version")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if version {
		gnp := gnparser.NewGNparser()
		fmt.Printf("\nversion: %s\n\nbuild:   %s\n\n",
			gnp.Version(), gnp.Build())
		os.Exit(0)
	}
}

func skipCleanupFlag(cmd *cobra.Command) bool {
	nocleanup, err := cmd.Flags().GetBool("nocleanup")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return nocleanup
}

func grpcFlag(cmd *cobra.Command) int {
	grpcPort, err := cmd.Flags().GetInt("grpc_port")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return grpcPort
}

func webFlag(cmd *cobra.Command) int {
	webPort, err := cmd.Flags().GetInt("web_port")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return webPort
}

func formatFlag(cmd *cobra.Command) string {
	str, err := cmd.Flags().GetString("format")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return str
}

func workersNumFlag(cmd *cobra.Command) int {
	i, err := cmd.Flags().GetInt("jobs")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return i
}

func processStdin(cmd *cobra.Command, jobs int, opts []gnparser.Option) {
	if !checkStdin() {
		_ = cmd.Help()
		return
	}
	gnp := gnparser.NewGNparser(opts...)
	parseFile(gnp, os.Stdin, jobs, opts)
}

func checkStdin() bool {
	stdInFile := os.Stdin
	stat, err := stdInFile.Stat()
	if err != nil {
		log.Panic(err)
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func getInput(cmd *cobra.Command, args []string) string {
	var data string
	switch len(args) {
	case 1:
		data = args[0]
	default:
		_ = cmd.Help()
		os.Exit(0)
	}
	return data
}

func parse(data string, jobs int, opts []gnparser.Option) {
	gnp := gnparser.NewGNparser(opts...)

	path := string(data)
	if fileExists(path) {
		f, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		parseFile(gnp, f, jobs, opts)
		f.Close()
	} else {
		parseString(gnp, data)
	}
}

func fileExists(path string) bool {
	if fi, err := os.Stat(path); err == nil {
		if fi.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func parseFile(gnp gnparser.GNparser, f io.Reader, jobs int,
	opts []gnparser.Option) {
	in := make(chan string)
	out := make(chan *gnparser.ParseResult)
	var wg sync.WaitGroup
	wg.Add(1)

	go gnparser.ParseStream(jobs, in, out, opts...)
	go processResults(gnp, out, &wg)
	sc := bufio.NewScanner(f)
	count := 0
	for sc.Scan() {
		count++
		if count%50000 == 0 {
			log.Printf("Parsing %d-th line\n", count)
		}
		name := sc.Text()
		in <- name
	}
	close(in)
	wg.Wait()
}

func processResults(gnp gnparser.GNparser, out <-chan *gnparser.ParseResult,
	wg *sync.WaitGroup) {
	defer wg.Done()
	if gnp.Format == gnparser.CSV {
		fmt.Println(output.CSVHeader())
	}
	for r := range out {
		if r.Error != nil {
			log.Println(r.Error)
		}
		fmt.Println(r.Output)
	}
}

func parseString(gnp gnparser.GNparser, data string) {
	res, err := gnp.ParseAndFormat(data)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	if gnp.Format == gnparser.CSV {
		fmt.Println(output.CSVHeader())
	}
	fmt.Println(res)
}
