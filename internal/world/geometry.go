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
	ID           string  `json:"id"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Width        float64 `json:"width"`
	Height       float64 `json:"height"`
	SolidSides   bool    `json:"solidSides,omitempty"`
	SolidCeiling bool    `json:"solidCeiling,omitempty"`
}

type Wall struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Ladder struct {
	ID         string  `json:"id"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	ClimbSpeed float64 `json:"climbSpeed,omitempty"`
}

type Polygon struct {
	ID     string  `json:"id"`
	Points []Point `json:"points"`
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

func PolygonBounds(polygon Polygon) (Rect, bool) {
	if len(polygon.Points) == 0 {
		return Rect{}, false
	}
	minX := polygon.Points[0].X
	maxX := polygon.Points[0].X
	minY := polygon.Points[0].Y
	maxY := polygon.Points[0].Y
	for _, point := range polygon.Points[1:] {
		if point.X < minX {
			minX = point.X
		}
		if point.X > maxX {
			maxX = point.X
		}
		if point.Y < minY {
			minY = point.Y
		}
		if point.Y > maxY {
			maxY = point.Y
		}
	}
	return Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}, true
}

func PolygonTopYAt(polygon Polygon, x float64) (float64, bool) {
	if len(polygon.Points) < 3 {
		return 0, false
	}
	found := false
	var topY float64
	for i := range polygon.Points {
		a := polygon.Points[i]
		b := polygon.Points[(i+1)%len(polygon.Points)]
		if a.X == b.X {
			continue
		}
		y, ok := segmentYAt(a, b, x)
		if !ok {
			continue
		}
		if !found || y < topY {
			topY = y
			found = true
		}
	}
	return topY, found
}

func RectOverlapsPolygon(rect Rect, polygon Polygon) bool {
	bounds, ok := PolygonBounds(polygon)
	if !ok || !RectsOverlap(rect, bounds) {
		return false
	}
	corners := []Point{
		{X: rect.X, Y: rect.Y},
		{X: rect.X + rect.Width, Y: rect.Y},
		{X: rect.X, Y: rect.Y + rect.Height},
		{X: rect.X + rect.Width, Y: rect.Y + rect.Height},
	}
	for _, corner := range corners {
		if PolygonContainsPoint(polygon, corner) {
			return true
		}
	}
	for _, point := range polygon.Points {
		if ContainsPoint(rect, point) {
			return true
		}
	}
	for i := range polygon.Points {
		a := polygon.Points[i]
		b := polygon.Points[(i+1)%len(polygon.Points)]
		if segmentIntersectsRect(a, b, rect) {
			return true
		}
	}
	return false
}

func PolygonContainsPoint(polygon Polygon, point Point) bool {
	inside := false
	count := len(polygon.Points)
	if count < 3 {
		return false
	}
	j := count - 1
	for i := 0; i < count; i++ {
		a := polygon.Points[i]
		b := polygon.Points[j]
		intersects := (a.Y > point.Y) != (b.Y > point.Y) &&
			point.X < (b.X-a.X)*(point.Y-a.Y)/(b.Y-a.Y)+a.X
		if intersects {
			inside = !inside
		}
		j = i
	}
	return inside
}

func segmentIntersectsRect(a Point, b Point, rect Rect) bool {
	if ContainsPoint(rect, a) || ContainsPoint(rect, b) {
		return true
	}
	edges := [][2]Point{
		{{X: rect.X, Y: rect.Y}, {X: rect.X + rect.Width, Y: rect.Y}},
		{{X: rect.X + rect.Width, Y: rect.Y}, {X: rect.X + rect.Width, Y: rect.Y + rect.Height}},
		{{X: rect.X + rect.Width, Y: rect.Y + rect.Height}, {X: rect.X, Y: rect.Y + rect.Height}},
		{{X: rect.X, Y: rect.Y + rect.Height}, {X: rect.X, Y: rect.Y}},
	}
	for _, edge := range edges {
		if segmentsIntersect(a, b, edge[0], edge[1]) {
			return true
		}
	}
	return false
}

func segmentsIntersect(a Point, b Point, c Point, d Point) bool {
	return orientation(a, c, d) != orientation(b, c, d) && orientation(a, b, c) != orientation(a, b, d)
}

func orientation(a Point, b Point, c Point) bool {
	return (c.Y-a.Y)*(b.X-a.X) > (b.Y-a.Y)*(c.X-a.X)
}

func min(a float64, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
