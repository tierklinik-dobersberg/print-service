package cups

type ColorMode string

const (
	ColorModeAuto      = ColorMode("auto")
	ColorModeColor     = ColorMode("color")
	ColorModeGrayScale = ColorMode("grayscale")
)

type Orientation string

const (
	OrientationPortrait  = Orientation("portrait")
	OrientationLandscape = Orientation("landscape")
)

const (
	AttributeLongRunningOperationID = "long-running-operation-id" // ipp.TagString
	AttributePrintColorMode         = "print-color-mode"          // ipp.TagKeyword
	AttributePrintColorModeDefault  = "print-color-mode-default"  // ipp.TagKeyword
)
