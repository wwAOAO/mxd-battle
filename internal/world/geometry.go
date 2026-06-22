package world

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Portal struct {
	ID           string `json:"id"`
	TargetRoomID string `json:"targetRoomId"`
	Area         Rect   `json:"area"`
	Target       Point  `json:"target"`
}

type Platform struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	//开启平台侧边碰撞
	SolidSides bool `json:"solidSides,omitempty"`
	//开启平台头顶碰撞
	SolidCeiling bool `json:"solidCeiling,omitempty"`
}

type Wall struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Terrain struct {
	ID     string  `json:"id"`
	X1     float64 `json:"x1,omitempty"`
	Y1     float64 `json:"y1,omitempty"`
	X2     float64 `json:"x2,omitempty"`
	Y2     float64 `json:"y2,omitempty"`
	Points []Point `json:"points,omitempty"`
}

func ContainsPoint(rect Rect, point Point) bool {
	return point.X >= rect.X &&
		point.X <= rect.X+rect.Width &&
		point.Y >= rect.Y &&
		point.Y <= rect.Y+rect.Height
}

func OverlapsVertical(aTop float64, aBottom float64, b Rect) bool {
	return aBottom > b.Y && aTop < b.Y+b.Height
}

func RectsOverlap(a Rect, b Rect) bool {
	return a.X < b.X+b.Width &&
		a.X+a.Width > b.X &&
		a.Y < b.Y+b.Height &&
		a.Y+a.Height > b.Y
}

func TerrainYAt(terrain Terrain, x float64) (float64, bool) {
	points := TerrainPoints(terrain)
	if len(points) < 2 {
		return 0, false
	}
	for i := 0; i < len(points)-1; i++ {
		y, ok := segmentYAt(points[i], points[i+1], x)
		if ok {
			return y, true
		}
	}
	return 0, false
}

func TerrainPoints(terrain Terrain) []Point {
	if len(terrain.Points) > 0 {
		return terrain.Points
	}
	return []Point{
		{X: terrain.X1, Y: terrain.Y1},
		{X: terrain.X2, Y: terrain.Y2},
	}
}

func segmentYAt(a Point, b Point, x float64) (float64, bool) {
	minX := a.X
	maxX := b.X
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	if x < minX || x > maxX {
		return 0, false
	}
	if a.X == b.X {
		return min(a.Y, b.Y), true
	}
	t := (x - a.X) / (b.X - a.X)
	return a.Y + (b.Y-a.Y)*t, true
}
func Clamp(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
