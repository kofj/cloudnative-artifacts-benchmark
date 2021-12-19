package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	raw_events   = []string{"Scheduled", "Pulling", "Pulled", "Created", "Started", "Ready"}
	count_events = []string{"Sandbox", "Pulled", "Created", "Started", "Ready"}
	kubeconfig   string
	output       string
	endline      string
	backend      string
	app          string
	caseindex    int64
	// date          string = time.Now().Local().Format("200601021504")
	letters   string = "ABCDEFGHIJKLMNOKQRSTUVWXYS"
	timeStyle string = "2006/01/02 15:04:05"
)

var (
	backendcol = map[string]string{
		"apparate": "B",
		"nydus":    "C",
		"stargz":   "D",
		"docker":   "E",
	}
	raw_data_col_offset = strings.Index(letters, "I")
	raw_data_tab_header = map[string]string{
		"I2": "Pods", "J2": "Scheduled", "K2": "Pulling", "L2": "Pulled", "M2": "Created", "N2": "Started", "O2": "Ready",
	}
	count_data_tab_header = map[string]string{
		"A2": "Pods", "B2": "Sandbox", "C2": "Pulled", "D2": "Created", "E2": "Started", "F2": "Ready", "G2": "Total",
	}
)

func init() {

	flag.StringVar(&kubeconfig, "kubeconfig", path.Join(".kube", "config"), "path to kubeconfig")
	flag.StringVar(&app, "app", "appname", "test case app name")
	flag.StringVar(&output, "output", "data/bench.xlsx", "output excel file path")
	flag.StringVar(&endline, "endline", "", "the line of container log meaning container finished")
	flag.StringVar(&backend, "backend", "nydus", "backend engine of container runtime")
	flag.Int64Var(&caseindex, "idx", 0, "case index")
	flag.Parse()
}

func main() {
	raw_data_table := map[string]map[string]time.Time{}
	count_data_table := map[string]map[string]time.Duration{}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err.Error())
	}
	logrus.Infoln("connected k8s cluster")
	var selector = fmt.Sprintf("app=%s", app)
	pl, err := clientset.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		log.Fatal(err.Error())
	}
	logrus.WithField("selector", selector).WithField("count", len(pl.Items)).Infoln("list pods")
	for _, pod := range pl.Items {
		logrus.WithField("pod", pod.Name).Infoln("collect events")
		events, err := clientset.CoreV1().Events("default").List(context.Background(), metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("involvedObject.name", pod.Name).String(),
		})
		if err != nil {
			log.Println(err.Error())
			continue
		}

		emap := make(map[string]time.Time)
		tmap := make(map[string]time.Duration)
		for _, e := range events.Items {
			// logrus.WithField("reason", e.Reason).WithField("time", e.FirstTimestamp).Errorln()
			switch e.Reason {
			case "Scheduled":
				if e.EventTime.IsZero() {
					emap[e.Reason] = e.FirstTimestamp.Time
				} else {
					emap[e.Reason] = e.EventTime.Time
				}

			default:
				if e.FirstTimestamp.IsZero() {
					emap[e.Reason] = e.EventTime.Time
				} else {
					emap[e.Reason] = e.FirstTimestamp.Time
				}
			}
		}

		// custom ready time via logs
		// logrus.WithField("pod", pod.Name).WithField("time", emap).Warnln("raw emap")
		if endline != "" {
			result, err := clientset.CoreV1().Pods("default").GetLogs(pod.Name, &v1.PodLogOptions{Timestamps: true}).Stream(context.Background())
			if err != nil {
				log.Printf("get log failed: %s\n", err.Error())
				continue
			}
			buffer, err := ioutil.ReadAll(result)
			if err != nil {
				log.Println(err.Error())
				continue
			}
			for _, line := range strings.Split(string(buffer), "\n") {
				if strings.Contains(line, endline) {
					t, _ := time.Parse(timeStyle, strings.Replace(strings.Split(line, " ")[0], "T", " ", 1))
					emap["Ready"] = t.UTC()
					break
				}
			}
			_ = result.Close()
		} else {
			// logrus.WithField("pod", pod.Name).Warnln("ready time from pod status")
			for _, cond := range pod.Status.Conditions {
				if cond.Type == "Ready" && cond.Status == "True" {
					emap["Ready"] = cond.LastTransitionTime.Time
				}
				if emap["Ready"].IsZero() {
					if cond.Type == "ContainersReady" && cond.Status == "True" {
						emap["Ready"] = cond.LastTransitionTime.Time
					}
				} else {
					break
				}
			}
			// logrus.WithField("pod", pod.Name).
			// 	WithField("Pulled", emap["Pulled"].Format("2006/01/02 15:04:05.0000")).
			// 	WithField("Ready", emap["Ready"].Format("2006/01/02 15:04:05.0000")).
			// 	Error("Time info")
		}

		raw_data_table[pod.Name] = emap
		// count time used
		for i := 0; i < len(count_events); i++ {
			tmap[count_events[i]] = emap[raw_events[i+1]].Sub(emap[raw_events[i]])
		}
		tmap["Total"] = emap[raw_events[len(raw_events)-1]].Sub(emap[raw_events[0]])
		count_data_table[pod.Name] = tmap
	}

	/******** START Write Excel ********/
	var sheet = fmt.Sprintf("%s %s", backend, app)
	var xlsx, err2 = excelize.OpenFile(output)
	if os.IsNotExist(err2) {
		xlsx, _ = excelize.OpenFile("data/template.xlsx")
	} else if err != nil {
		log.Panicf("open excel failed: %s", err2)
	}

	// 1. Write case table headers
	var exampleIndex = xlsx.GetSheetIndex("example")
	var newIndex = xlsx.NewSheet(sheet)
	xlsx.CopySheet(exampleIndex, newIndex)
	xlsx.SetActiveSheet(exampleIndex)

	for k, v := range raw_data_tab_header {
		xlsx.SetCellValue(sheet, k, v)
	}
	for k, v := range count_data_tab_header {
		xlsx.SetCellValue(sheet, k, v)
	}

	// 2. write raw data
	var line = 3
	for name, cols := range raw_data_table {
		xlsx.SetCellValue(sheet, fmt.Sprintf("I%d", line), name)
		for col, event := range raw_events {
			var axis = fmt.Sprintf("%c%d", letters[col+raw_data_col_offset+1], line)
			var vaule = cols[event].UTC()
			// logrus.WithField("Axis", axis).WithField("Pod", name).WithField("Vaule", vaule).Infoln()
			xlsx.SetCellValue(sheet, axis, vaule)
		}
		line++
	}

	// 3. write count data
	line = 3
	for name, cols := range count_data_table {
		xlsx.SetCellValue(sheet, fmt.Sprintf("A%d", line), name)
		logrus.WithField("Pod", name).WithField("Vaule", cols).Infoln()
		for colidx, event := range count_events {
			var axis = fmt.Sprintf("%c%d", letters[colidx+1], line)
			xlsx.SetCellValue(sheet, axis, cols[event].Milliseconds())
		}
		xlsx.SetCellValue(sheet, fmt.Sprintf("G%d", line), cols["Total"].Milliseconds())
		line++
	}

	// 4. write summary table
	xlsx.SetCellValue("Summary", fmt.Sprintf("A%d", caseindex+3), app)

	var axis = fmt.Sprintf("%s%d", backendcol[backend], caseindex+3)
	var formula = fmt.Sprintf("AVERAGE('%s'!G3:G52)", sheet)
	xlsx.SetCellFormula("Summary", axis, formula)

	// 5. Save to file
	if err = xlsx.SaveAs(output); err != nil {
		fmt.Println(err)
	}

	/******** END Write Excel ********/
}
