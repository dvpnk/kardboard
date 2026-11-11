import AppKit

// The geometry below is identical to kardboard.svg, in its 32-unit viewBox.
let destination = URL(fileURLWithPath: CommandLine.arguments[1])
func mark(_ c: CGContext) {
    c.setFillColor(NSColor(calibratedWhite: 17.0 / 255, alpha: 1).cgColor)
    c.fill(CGRect(x: 2, y: 2, width: 6, height: 28))
    let p = CGMutablePath()
    p.move(to: CGPoint(x: 24, y: 2))
    p.addLine(to: CGPoint(x: 30, y: 2))
    p.addLine(to: CGPoint(x: 30, y: 30))
    p.addLine(to: CGPoint(x: 24, y: 30))
    p.addCurve(to: CGPoint(x: 6, y: 16), control1: CGPoint(x: 22, y: 25), control2: CGPoint(x: 6, y: 23))
    p.addCurve(to: CGPoint(x: 24, y: 2), control1: CGPoint(x: 6, y: 9), control2: CGPoint(x: 22, y: 7))
    p.closeSubpath()
    p.move(to: CGPoint(x: 24, y: 8))
    p.addCurve(to: CGPoint(x: 12, y: 16), control1: CGPoint(x: 20, y: 12), control2: CGPoint(x: 12, y: 12))
    p.addCurve(to: CGPoint(x: 24, y: 24), control1: CGPoint(x: 12, y: 20), control2: CGPoint(x: 20, y: 20))
    p.closeSubpath()
    c.addPath(p)
    c.drawPath(using: .eoFill)
}
func canvas(_ w: Int, _ h: Int) -> CGContext {
    CGContext(data: nil, width: w, height: h, bitsPerComponent: 8, bytesPerRow: w * 4,
              space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
}
func save(_ c: CGContext, _ name: String) throws {
    let rep = NSBitmapImageRep(cgImage: c.makeImage()!)
    try rep.representation(using: .png, properties: [:])!.write(to: destination.appendingPathComponent(name))
}
var icons: [Int: CGImage] = [:]
for size in [16, 32, 512] {
    let c = canvas(size, size)
    c.scaleBy(x: CGFloat(size) / 32, y: CGFloat(size) / 32)
    mark(c)
    try save(c, size == 512 ? "kardboard-512.png" : "favicon-\(size).png")
    icons[size] = c.makeImage()!
}
let c = canvas(960, 570)
c.setFillColor(NSColor(calibratedRed: 0.98, green: 0.97, blue: 0.94, alpha: 1).cgColor)
c.fill(CGRect(x: 0, y: 0, width: 960, height: 570))
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(cgContext: c, flipped: false)
func label(_ text: String, _ x: CGFloat, _ y: CGFloat, _ size: CGFloat = 16) {
    (text as NSString).draw(at: CGPoint(x: x, y: y), withAttributes: [
        .font: NSFont.monospacedSystemFont(ofSize: size, weight: .regular),
        .foregroundColor: NSColor(calibratedWhite: 0.15, alpha: 1)])
}
label("kardboard", 48, 510, 28)
label("SVG", 100, 442)
c.saveGState()
c.translateBy(x: 58, y: 150)
c.scaleBy(x: 8, y: 8)
mark(c)
c.restoreGState()
for (index, size) in [16, 32].enumerated() {
    let x = CGFloat(400 + index * 280)
    label("\(size) × \(size) px", x, 442)
    c.interpolationQuality = .none
    c.draw(icons[size]!, in: CGRect(x: x, y: 388, width: CGFloat(size), height: CGFloat(size)))
    label("Original", x, 354, 13)
    c.draw(icons[size]!, in: CGRect(x: x, y: 145, width: 192, height: 192))
    label("\(192 / size)× · Pixelansicht", x, 112, 13)
}
label("Echte PNG-Rasterung · transparente Favicons", 48, 42, 14)
NSGraphicsContext.restoreGraphicsState()
try save(c, "favicon-preview.png")
