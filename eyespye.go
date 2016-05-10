package main

import (
    "gopkg.in/gographics/imagick.v1/imagick"
    "fmt"
    "sync"
    "runtime"
    "os"
    "time"
    "encoding/json"
)

func makeTimestamp() int64 {
    return time.Now().UnixNano()
}

type ImageInfo struct {
    Filename string
    Coverage float64
    Background float64
    BackgroundColor string
    Height int
    Width int
}

func BlackOrWhiteBackground(mw *imagick.MagickWand, width int, height int, fuzz float64) string {

    var pixIterators []*imagick.PixelIterator

    //get corner 10x10 regions of the picture
    pixIterators = append(pixIterators, mw.NewPixelRegionIterator(0, 0, 10, 10))
    pixIterators = append(pixIterators, mw.NewPixelRegionIterator(width -10, 0, 10, 10))
    pixIterators = append(pixIterators, mw.NewPixelRegionIterator(0, height - 10, 10, 10))
    pixIterators = append(pixIterators, mw.NewPixelRegionIterator(width - 10, height - 10, 10, 10))

    white, black := imagick.NewPixelWand(), imagick.NewPixelWand()
    white.SetColor("white")
    black.SetColor("black")
    whiteCount, blackCount := 0, 0

    //Match the pixel rows either to black or white
    for _, iterator := range pixIterators {
        for _, pixelRow := range iterator.GetNextIteratorRow() {
            //fmt.Println(pixelRow.GetColorAsString())
            if pixelRow.IsSimilar(white, fuzz) {
                whiteCount++
            } else if pixelRow.IsSimilar(black, fuzz) {
                blackCount++
            }
        }
    }

    //fmt.Println("Black count:", blackCount, " White count:", whiteCount)

    if blackCount > whiteCount && blackCount > 0 {
        return "black"
    } else {
        return "white"
    }
}

func GetCoverage(mw *imagick.MagickWand, backgroundColorAsString string, width int, height int, fuzz float64) float64{

    // fillcolor and bordercolor
    transparentColor, bc := imagick.NewPixelWand(), imagick.NewPixelWand()
    transparentColor.SetColor("none")
    bc.SetColor(backgroundColorAsString)

    rgba := imagick.CHANNELS_RGB | imagick.CHANNEL_ALPHA
    //fill from all corners +-1 pixels
    mw.FloodfillPaintImage(rgba, transparentColor, fuzz, bc, width-1, height-1, false)
    mw.FloodfillPaintImage(rgba, transparentColor, fuzz, bc, 1, 1, false)
    mw.FloodfillPaintImage(rgba, transparentColor, fuzz, bc, width-1, 1, false)
    mw.FloodfillPaintImage(rgba, transparentColor, fuzz, bc, 1, height-1, false)

    mw.LevelImage(0.9, 0, 1)

    number, pixels := mw.GetImageHistogram()
    pixels = pixels[:number];
    transparent, allcount := uint(0), uint(0)

    for _, pix := range pixels {
        if pix.IsVerified() == true {
            if pix.GetAlpha() == 0 {
                transparent = transparent + pix.GetColorCount()
            }
            allcount = allcount + pix.GetColorCount()
        }
    }

    coverage := 100 - (float64(transparent) / float64(allcount) * 100)

    return coverage
}

func GetBackgroundInfo(filePath string, c chan ImageInfo, wg *sync.WaitGroup) {

    var err error
    wroteToChannel := false;

    defer func() {
        if (!wroteToChannel) {
            wg.Done()
        }
    }()

    mw := imagick.NewMagickWand()
    err = mw.ReadImage(filePath)
    if err != nil {
        panic(err)
    }

    // Get original logo size
    width := int(mw.GetImageWidth())
    height := int(mw.GetImageHeight())

    backgroundColorString := BlackOrWhiteBackground(mw, width, height, 200)
    coverage := GetCoverage(mw, backgroundColorString, width, height, 1500)

    c <- ImageInfo{filePath, coverage, 100 - coverage, backgroundColorString, height, width}
    wroteToChannel = true
}

func GetBackgroundInfoSync(filePath string) ImageInfo {

    mw := imagick.NewMagickWand()
    err := mw.ReadImage(filePath)
    if err != nil {
        panic(err)
    }

    // Get original logo size
    width := int(mw.GetImageWidth())
    height := int(mw.GetImageHeight())

    backgroundColorString := BlackOrWhiteBackground(mw, width, height, 200)
    coverage := GetCoverage(mw, backgroundColorString, width, height, 1500)

    //mw.DisplayImage(os.Getenv("DISPLAY"))
    return ImageInfo{filePath, coverage, 100 - coverage, backgroundColorString, height, width}

}

func main() {
    var wg sync.WaitGroup
    files := os.Args[1:]
    showStats := true
    asynchronous := true

    //files = []string{
    //    "/home/luka/Desktop/IMAGES/25percentWithHoleInTheMiddle.jpg",
    //    "/home/luka/Desktop/IMAGES/25percent.jpg",
    //    "/home/luka/Desktop/IMAGES/25percentUpperCorner.jpg",
    //    "/home/luka/Desktop/IMAGES/whiteSquare.png",
    //    "/home/luka/Desktop/IMAGES/a.jpg",
    //    "/home/luka/Desktop/IMAGES/b.jpg",
    //    "/home/luka/Desktop/IMAGES/c.png",
    //    "/home/luka/Desktop/IMAGES/d.jpg",
    //    "/home/luka/Desktop/IMAGES/e.png",
    //    "/home/luka/Desktop/IMAGES/f.jpg",
    //    "/home/luka/Desktop/IMAGES/g.jpg",
    //    "/home/luka/Desktop/IMAGES/i.jpg",
    //    "/home/luka/Desktop/IMAGES/blackBackground.jpg",
    //    "/home/luka/Desktop/IMAGES/transparent.png",
    //    "/home/luka/Desktop/IMAGES/transparentWhite.png",
    //}

    var data []ImageInfo
    channel := make(chan ImageInfo)
    imagick.Initialize()
    startMark := makeTimestamp()

    if (asynchronous) {
        wg.Add(len(files))
        //run a routine for each file
        for _, file := range files {
            go GetBackgroundInfo(file, channel, &wg)
        }

        //Collect ImageInfo objects and mark routines as done
        go func() {
            defer wg.Done()
            for response := range channel {
                data = append(data, response)
                wg.Done()
            }
        }()

        wg.Wait()

    } else {
        for _, file := range files {
            data = append(data, GetBackgroundInfoSync(file))
        }
    }


    endMark := makeTimestamp()

    json, _ := json.Marshal(data)
    fmt.Println(string(json))

    for _, info := range data {
        fmt.Println(info.Filename, info.Coverage)
    }

    if (showStats) {
        s := new(runtime.MemStats)
        runtime.ReadMemStats(s)
        fmt.Println("Alloc : ", s.Alloc /1024 , "MB")
        fmt.Println("Total Alloc : ", s.TotalAlloc  /1024 , "MB")
        fmt.Println("Sys : ", s.Sys  /1024 , "MB")
        fmt.Println("Lookups : ", s.Lookups)
        duration := (endMark - startMark) / (int64(time.Millisecond)/int64(time.Nanosecond))
        fmt.Println("Runtime:", duration,"ms")
    }

}
