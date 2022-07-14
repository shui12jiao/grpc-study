package sample

import (
	"math/rand"
	"pcbook/pb"
	"time"

	"github.com/google/uuid"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

//Keyboard
func randomKeyboardLayout() pb.Keyboard_Layout {
	switch rand.Intn(3) {
	case 1:
		return pb.Keyboard_QWERTY
	case 2:
		return pb.Keyboard_QWERTZ
	default:
		return pb.Keyboard_AZERTY
	}
}

func randomBool() bool {
	return rand.Intn(2) == 1
}

//CPU
func randomCPUBrand() string {
	return randomStringFromSet("Intel", "AMD")
}

func randomCPUName(brand string) string {
	if brand == "Intel" {
		return randomStringFromSet("Xeon E-2286M", "Core i5-4570", "Core i7-12900K", "Core i3-10100")
	}
	return randomStringFromSet("Ryzen 7 3700X", "Ryzen 5 3500U", "Ryzen 3 3200GE", "Ryzen ThreadRipper Pro 3955WX")
}

//GPU
func randomGPUBrand() string {
	return randomStringFromSet("NVIDIA", "AMD")
}

func randomGPUName(brand string) string {
	if brand == "NVIDIA" {
		return randomStringFromSet("GTX 1070", "GTX 1660Ti", "GTX 1060", "RTX 2070")
	}
	return randomStringFromSet("RX 580", "RX 5700 XT", "RX Vega 56", "RX 6600 XT")
}

//Screen
func randomScreenResolution() *pb.Screen_Resolution {
	height := randomInt(1080, 4320)
	width := height * 16 / 9

	resolution := &pb.Screen_Resolution{
		Height: uint32(height),
		Width:  uint32(width),
	}

	return resolution
}

func randomScreenPanel() pb.Screen_Panel {
	switch rand.Intn(4) {
	case 1:
		return pb.Screen_IPS
	case 2:
		return pb.Screen_VA
	case 3:
		return pb.Screen_TN
	default:
		return pb.Screen_OLED
	}
}

//Laptop
func randomLaptopBrand() string {
	return randomStringFromSet("Apple", "Dell", "Lenovo", "HP")
}

func randomLaptopName(brand string) string {
	switch brand {
	case "Apple":
		return randomStringFromSet("Macbook air", "Macbook Pro")
	case "Dell":
		return randomStringFromSet("Latitude", "Vostro", "XPS", "Alienware")
	case "Lenovo":
		return randomStringFromSet("Thinkpad X1", "Legion")
	default:
		return randomStringFromSet("Victus", "ZHAN 66")
	}
}

func randomStringFromSet(a ...string) string {
	n := len(a)
	if n == 0 {
		return ""
	}
	return a[rand.Intn(n)]
}

func randomID() string {
	return uuid.New().String()
}

func randomInt(min, max int) int {
	return min + rand.Intn(max-min+1)
}

func randomFloat64(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

func randomFloat32(min, max float32) float32 {
	return min + rand.Float32()*(max-min)
}
