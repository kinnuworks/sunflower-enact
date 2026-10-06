// Sunflower moves a Deployment to the node the ENACT policy operator has chosen for it.
//
// The operator keeps a RuntimePolicy's status.chosenNode up to date but has no rights to
// touch workloads, and the ENACT SDK pins a Deployment to that node only once, at deploy
// time. Sunflower is the missing last step: it watches the choice and carries it out,
// starting the new pod before stopping the old one, and only when the move is justified.
package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kinnuworks/sunflower-enact/sunflower/internal/controller"
	"github.com/kinnuworks/sunflower-enact/sunflower/internal/placement"
)

func main() {
	s := placement.DefaultSettings()
	flag.DurationVar(&s.Settle, "settle", s.Settle, "how long a reason to move must last before acting")
	flag.DurationVar(&s.Dwell, "dwell", s.Dwell, "minimum time between two moves of the same app")
	flag.Float64Var(&s.Margin, "margin", s.Margin, "how much greener (0..1) the chosen node must be to leave a node that still satisfies the policy")
	flag.IntVar(&s.MaxMovesPerHour, "max-moves-per-hour", s.MaxMovesPerHour, "cap on moves per app per hour")
	moveTimeout := flag.Duration("move-timeout", 90*time.Second, "undo a move that has not completed in this time")
	dryRun := flag.Bool("dry-run", false, "report what would happen without changing anything")
	receiptsAddr := flag.String("receipts-addr", ":8085", "address serving the log of actions as JSON at /receipts")
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("sunflower")

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		log.Error(err, "cannot start")
		os.Exit(1)
	}
	receipts := controller.NewReceipts(500)
	go func() {
		mux := http.NewServeMux()
		mux.Handle("GET /receipts", receipts)
		if err := http.ListenAndServe(*receiptsAddr, mux); err != nil {
			log.Error(err, "receipts endpoint stopped")
		}
	}()
	r := &controller.Reconciler{
		Receipts:    receipts,
		Client:      mgr.GetClient(),
		Recorder:    mgr.GetEventRecorderFor("sunflower"),
		Settings:    s,
		MoveTimeout: *moveTimeout,
		DryRun:      *dryRun,
	}
	if err := r.SetupWithManager(mgr); err != nil {
		log.Error(err, "cannot set up the controller")
		os.Exit(1)
	}
	log.Info("watching Deployments annotated with "+controller.PolicyAnnotation,
		"settle", s.Settle, "dwell", s.Dwell, "margin", s.Margin, "dryRun", *dryRun)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "stopped")
		os.Exit(1)
	}
}
