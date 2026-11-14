import AppKit
// Reproducible geometric app icon: two shelves inside a pair of braces.
let size = NSSize(width: 1024, height: 1024)
let image = NSImage(size: size)
image.lockFocus()
NSColor(calibratedRed: 0.08, green: 0.15, blue: 0.20, alpha: 1).setFill()
NSBezierPath(roundedRect: NSRect(x: 70, y: 70, width: 884, height: 884), xRadius: 196, yRadius: 196).fill()
NSColor(calibratedRed: 0.33, green: 0.84, blue: 0.76, alpha: 1).setStroke()
func line(_ points: [NSPoint], width: CGFloat) {
 let path = NSBezierPath(); path.lineWidth = width; path.lineCapStyle = .round; path.lineJoinStyle = .round
 path.move(to: points[0]); for point in points.dropFirst() { path.line(to: point) }; path.stroke()
}
line([NSPoint(x: 345,y: 735),NSPoint(x: 300,y: 735),NSPoint(x: 300,y: 560),NSPoint(x: 250,y: 512),NSPoint(x: 300,y: 464),NSPoint(x: 300,y: 289),NSPoint(x: 345,y: 289)],width: 38)
line([NSPoint(x: 679,y: 735),NSPoint(x: 724,y: 735),NSPoint(x: 724,y: 560),NSPoint(x: 774,y: 512),NSPoint(x: 724,y: 464),NSPoint(x: 724,y: 289),NSPoint(x: 679,y: 289)],width: 38)
NSColor.white.setStroke()
line([NSPoint(x: 414,y: 420),NSPoint(x: 610,y: 420)],width: 35)
line([NSPoint(x: 414,y: 590),NSPoint(x: 610,y: 590)],width: 35)
image.unlockFocus()
let bitmap = NSBitmapImageRep(data: image.tiffRepresentation!)!
try bitmap.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: "resources/icon.png"))
