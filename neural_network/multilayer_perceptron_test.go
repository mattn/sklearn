package neuralnetwork

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/pa-m/sklearn/base"
	"github.com/pa-m/sklearn/datasets"
	"github.com/pa-m/sklearn/metrics"
	modelselection "github.com/pa-m/sklearn/model_selection"
	"github.com/pa-m/sklearn/pipeline"
	"github.com/pa-m/sklearn/preprocessing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

type Problem struct {
	X, Y          *mat.Dense
	MiniBatchSize int
}

func NewRandomProblem(nSamples, nFeatures, nOutputs int, activation string, loss string) *Problem {
	X := mat.NewDense(nSamples, nFeatures, nil)
	X.Apply(func(i, j int, v float64) float64 {
		return rand.Float64()
	}, X)
	TrueTheta := mat.NewDense(nFeatures, nOutputs, nil)
	TrueTheta.Apply(func(i, j int, v float64) float64 {
		return rand.Float64()
	}, TrueTheta)
	Z := mat.NewDense(nSamples, nOutputs, nil)
	Ytrue := mat.NewDense(nSamples, nOutputs, nil)
	Z.Mul(X, TrueTheta)
	NewActivation(activation).Func(Z, Ytrue)
	if loss == "cross-entropy" {
		for sample := 0; sample < nSamples; sample++ {
			oTrue := floats.MaxIdx(Ytrue.RawRowView(sample))
			for o := 0; o < nOutputs; o++ {
				y := 0.
				if o == oTrue {
					y = 1
				}
				Ytrue.Set(sample, o, y)
			}
		}
	}
	return &Problem{X: X, Y: Ytrue}
}

func TestMLPRegressorIdentitySquareLoss(t *testing.T) {
	testMLPRegressor(t, "identity", "square", "adam", 2)
}

func TestMLPRegressorLogisticLogLoss(t *testing.T) {
	testMLPRegressor(t, "logistic", "log", "adam", 2)
}

func TestMLPRegressorLogisticCrossEntropyLoss(t *testing.T) {
	testMLPRegressor(t, "logistic", "cross-entropy", "adam", 2)
}

// // tanh has values in -1..1 so cross entropy must be adapted
// func TestMLPRegressorTanhCrossEntropyLoss(t *testing.T) {
// 	testMLPRegressor(t, "tanh", "cross-entropy", "adam", 2)
// }

// func TestMLPRegressorReLUCrossEntropyLoss(t *testing.T) {
// 	testMLPRegressor(t, "relu", "cross-entropy", "adam", 2)
// }

func testMLPRegressor(t *testing.T, activationName string, lossName string, solver string, maxLayers int) {
	var nSamples, nFeatures, nOutputs = 2000, 3, 2
	//activation := base.Activations[activationName]
	var p = NewRandomProblem(nSamples, nFeatures, nOutputs, activationName, lossName)
	var HiddenLayerSizes []int

	for l := 0; l < maxLayers; l++ {
		Alpha := 1e-14
		regr := NewMLPRegressor(HiddenLayerSizes, activationName, solver, Alpha)

		// regr.SetOptimizer(func() Optimizer {
		// 	optimizer := base.NewAdamOptimizer()
		// 	optimizer.StepSize = 0.1
		// 	return optimizer
		// })

		//regr.SetOptimizer(OptimCreator, true)
		regr.Epochs = 40
		regr.GradientClipping = 5.
		testSetup := fmt.Sprintf("%T %s %s loss layers %v", regr, activationName, lossName, HiddenLayerSizes)
		//Ypred := mat.NewDense(nSamples, nOutputs, nil)

		regr.Loss = lossName
		start := time.Now()
		regr.Fit(p.X, p.Y)
		elapsed := time.Since(start)
		unused(elapsed)

		if regr.J > 0.01 && regr.J > 0.5*regr.JFirst {
			t.Errorf("%s JFirst=%g J=%g", testSetup, regr.JFirst, regr.J)
		} else {
			//			fmt.Printf("%s ok J=%g elapsed=%s\n", testSetup, regr.J, elapsed)
		}
		HiddenLayerSizes = append(HiddenLayerSizes, 1+rand.Intn(9))
	}
}

func TestMLPClassifierMicrochip(t *testing.T) {
	X, Ytrue := datasets.LoadMicroChipTest()
	nSamples, nFeatures := X.Dims()

	//Xp, _ := preprocessing.NewPolynomialFeatures(6).Fit(X, Ytrue).Transform(X, Ytrue)
	// add poly features manually to have same order
	Xp := mat.NewDense(nSamples, 27, nil)
	c := 0
	for i := 1; i <= 6; i++ {
		for j := 0; j <= i; j++ {
			for s := 0; s < nSamples; s++ {
				Xp.Set(s, c, math.Pow(X.At(s, 0), float64(i-j))*math.Pow(X.At(s, 1), float64(j)))
			}
			c++
		}
	}

	_, nFeatures = Xp.Dims()
	_, nOutputs := Ytrue.Dims()
	regr := NewMLPClassifier([]int{}, "logistic", "adam", 1.)
	//regr.Loss = "cross-entropy"

	// we allocate Coef here because we use it for loss and grad tests before Fit
	regr.allocLayers(nFeatures, nOutputs, func() float64 { return 0. })

	Ypred := mat.NewDense(nSamples, nOutputs, nil)
	var J float64
	loss := func() float64 {
		regr.forward(Xp, Ypred)
		return regr.fitEpoch(Xp, Ytrue, 1)
	}
	chkLoss := func(context string, expectedLoss float64) {
		if math.Abs(J-expectedLoss) > 1e-3 {
			t.Errorf("%s J=%g expected:%g", context, J, expectedLoss)
		}
	}
	chkGrad := func(context string, expectedGradient []float64) {
		actualGradient := regr.Layers[0].Grad.RawRowView(0)[0:len(expectedGradient)]

		//fmt.Printf("%s grad=%v expected %v\n", context, actualGradient, expectedGradient)
		for j := 0; j < len(expectedGradient); j++ {
			if !floats.EqualWithinAbs(expectedGradient[j], actualGradient[j], 1e-4) {
				t.Errorf("%s grad=%v expected %v", context, actualGradient, expectedGradient)
				return
			}
		}
	}

	J = loss()
	chkLoss("Microchip initial loss", 0.693)
	chkGrad("Microchip initial gradient", []float64{0.0085, 0.0188, 0.0001, 0.0503, 0.0115})

	regr.Layers[0].Theta.Copy(base.MatApply0{Rows: 1 + nFeatures, Columns: nOutputs, Func: func() float64 { return 1 }})
	regr.Alpha = 10.

	J = loss()
	chkLoss("At test theta", 3.164)
	chkGrad("at test theta", []float64{0.3460, 0.1614, 0.1948, 0.2269, 0.0922})

	best := make(map[string]string)
	bestLoss := math.Inf(1)
	bestTime := time.Second * 86400

	// // test Fit with various base.Optimizer
	var Optimizers = []string{
		"sgd",
		"adagrad",
		"rmsprop",
		"adadelta",
		"adam",
		"lbfgs",
	}
	newOptimizer := func(name string) base.Optimizer {

		switch name {
		case "adadelta":
			s := base.NewAdadeltaOptimizer()
			//s.StepSize = 0.1
			return s
		case "adam":
			s := base.NewAdamOptimizer()
			//s.StepSize = 2
			return s
		case "lbfgs":
			return nil
		default:
			s := base.NewOptimizer(name)
			return s
		}
	}
	//panic(fmt.Errorf("nSamples=%d", nSamples))
	for _, optimizer := range Optimizers {
		regr.Layers[0].Theta.Apply(func(feature, output int, _ float64) float64 { return 0. }, regr.Layers[0].Theta)

		testSetup := optimizer
		start := time.Now()
		regr.Alpha = 1.
		//regr.WeightDecay = .1
		regr.Epochs = 50
		regr.MiniBatchSize = 118 //1,2,59,118
		regr.Solver = optimizer
		switch optimizer {
		case "lbfgs":
			regr.Layers[0].Optimizer = nil
		default:
			regr.Layers[0].Optimizer = newOptimizer(optimizer)
		}

		regr.Fit(Xp, Ytrue)
		elapsed := time.Since(start)
		J = loss()
		//fmt.Println(testSetup, "elapsed time", elapsed, "loss", J)

		if J < bestLoss {
			bestLoss = J
			best["best for loss"] = testSetup + fmt.Sprintf("(%g)", J)
		}
		if elapsed < bestTime {
			bestTime = elapsed
			best["best for time"] = testSetup + fmt.Sprintf("(%s)", elapsed)
		}
		regr.Predict(Xp, Ypred)
		accuracy := metrics.AccuracyScore(Ytrue, Ypred, true, nil)
		// FIXME accuracy should be over 0.83
		expectedAccuracy := 0.82
		if accuracy < expectedAccuracy {
			t.Errorf("%s accuracy=%g expected:%g", optimizer, accuracy, expectedAccuracy)
		}
	}
	fmt.Println("MLPClassifier BEST SETUP:", best)

	// // fmt.Println("acc:", metrics.AccuracyScore(Ytrue, Ypred,true,nil))
	// fmt.Println("ok")
}

func TestMnist(t *testing.T) {
	X, Y := datasets.LoadMnist()

	X, Ybin := (&preprocessing.LabelBinarizer{}).FitTransform(X, Y)
	Theta1, Theta2 := datasets.LoadMnistWeights()
	mlp := NewMLPClassifier([]int{25}, "logistic", "adam", 0.)
	mlp.Loss = "cross-entropy"
	mlp.MiniBatchSize = 5000
	mlp.Shuffle = false
	mlp.allocLayers(400, 10, func() float64 { return 0. })
	mlp.Layers[0].Theta.Copy(Theta1.T())
	mlp.Layers[1].Theta.Copy(Theta2.T())
	J := mlp.fitEpoch(X, Ybin, 0)

	//fmt.Println("at test thetas J:=", J)
	// check cost at loaded theta is 0.287629
	if !floats.EqualWithinAbs(0.287629, J, 1e-6) {
		t.Errorf("Expected cost: %g, got %g", 0.287629, J)
	}

	mlp.Alpha = 1.
	mlp.Layers[0].Theta.Copy(Theta1.T())
	mlp.Layers[1].Theta.Copy(Theta2.T())
	J = mlp.fitEpoch(X, Ybin, 0)
	if !floats.EqualWithinAbs(0.383770, J, 1e-6) {
		t.Errorf("Expected cost: %g, got %g", 0.383770, J)
	}
}

func BenchmarkMnist(b *testing.B) {

	X, Y := datasets.LoadMnist()

	X, Ybin := (&preprocessing.LabelBinarizer{}).FitTransform(X, Y)
	Theta1, Theta2 := datasets.LoadMnistWeights()
	mlp := NewMLPClassifier([]int{25}, "logistic", "adam", 0.)
	mlp.Loss = "cross-entropy"
	mlp.MiniBatchSize = 5000
	mlp.Shuffle = false
	mlp.allocLayers(400, 10, func() float64 { return 0. })
	mlp.Layers[0].Theta.Copy(Theta1.T())
	mlp.Layers[1].Theta.Copy(Theta2.T())
	mlp.fitEpoch(X, Ybin, 0)
	b.ResetTimer()
	for epoch := 1; epoch < b.N*1; epoch++ {
		mlp.fitEpoch(X, Ybin, epoch)
	}

}

//go test ./neural_network -run BenchmarkMnist -bench ^BenchmarkMnist -cpuprofile /tmp/cpu.prof -memprofile /tmp/mem.prof -benchmem
//BenchmarkMnist-12            100          17387518 ns/op           89095 B/op         30 allocs/op

func ExampleMLPClassifier() {
	ds := datasets.LoadBreastCancer()
	fmt.Println("Dims", base.MatDimsString(ds.X, ds.Y))

	scaler := preprocessing.NewStandardScaler()
	X0, Y0 := scaler.Fit(ds.X, ds.Y).Transform(ds.X, ds.Y)
	nSamples, nOutputs := Y0.Dims()
	pca := preprocessing.NewPCA()
	X1, Y1 := pca.Fit(X0, Y0).Transform(X0, Y0)
	thres := .995
	ExplainedVarianceRatio := 0.
	var nComponents int
	for nComponents = 0; nComponents < len(pca.ExplainedVarianceRatio) && ExplainedVarianceRatio < thres; nComponents++ {
		ExplainedVarianceRatio += pca.ExplainedVarianceRatio[nComponents]
	}
	fmt.Printf("ExplainedVarianceRatio %.3f %.3f\n", ExplainedVarianceRatio, pca.ExplainedVarianceRatio[0:nComponents])
	fmt.Printf("%d components explain %.2f%% of variance\n", nComponents, thres*100.)
	X1 = base.MatDenseSlice(X1, 0, nSamples, 0, nComponents)

	poly := preprocessing.NewPolynomialFeatures(2)
	poly.IncludeBias = false
	X2, Y2 := poly.Fit(X1, Y1).Transform(X1, Y1)

	m := NewMLPClassifier([]int{}, "relu", "adam", 0.)
	//m.WeightDecay = 0.005
	m.Loss = "cross-entropy"

	m.Epochs = 100
	m.Fit(X2, Y2)
	Ypred := mat.NewDense(nSamples, nOutputs, nil)
	m.Predict(X2, Ypred)

	accuracy := metrics.AccuracyScore(Y2, Ypred, true, nil)
	fmt.Println("accuracy>0.994 ?", accuracy > 0.994)
	if accuracy <= 0.994 {
		fmt.Println("accuracy:", accuracy)
	}
	// Output:
	// Dims  569,30 569,1
	// ExplainedVarianceRatio 0.996 [0.443 0.190 0.094 0.066 0.055 0.040 0.023 0.016 0.014 0.012 0.010 0.009 0.008 0.005 0.003 0.003 0.002 0.002 0.002 0.001]
	// 20 components explain 99.50% of variance
	// accuracy>0.994 ? true

}

func ExampleMLPRegressor() {
	// exmaple inspired from # https://machinelearningmastery.com/regression-tutorial-keras-deep-learning-library-python/
	// with wider_model
	// added weight decay and reduced epochs from 100 to 20
	ds := datasets.LoadBoston()
	X, Y := ds.X, ds.Y
	mlp := NewMLPRegressor([]int{20}, "relu", "adam", 9.55e-5)
	mlp.WeightDecay = .1
	mlp.Shuffle = false
	mlp.MiniBatchSize = 5
	mlp.Epochs = 20
	m := pipeline.NewPipeline(
		pipeline.NamedStep{Name: "standardize", Step: preprocessing.NewStandardScaler()},
		pipeline.NamedStep{Name: "mlpregressor", Step: mlp},
	)
	_ = m
	randomState := modelselection.RandomState(7)
	scorer := func(Y, Ypred *mat.Dense) float64 {
		e := metrics.MeanSquaredError(Y, Ypred, nil, "").At(0, 0)
		return e
	}
	mean := func(x []float64) float64 { return floats.Sum(x) / float64(len(x)) }

	res := modelselection.CrossValidate(m, X, Y,
		nil,
		scorer,
		&modelselection.KFold{NSplits: 10, Shuffle: true, RandomState: &randomState}, 10)
	fmt.Println(math.Sqrt(mean(res.TestScore)) < 20)

	// Output:
	// true
}
