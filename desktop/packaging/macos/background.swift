// Run from the repository root:
// swift desktop/packaging/macos/background.swift desktop/packaging/macos/background.png
import AppKit

guard CommandLine.arguments.count == 2 else {
    fatalError("Usage: swift background.swift <output.png>")
}

let width = 660
let height = 440
let bitmap = NSBitmapImageRep(
    bitmapDataPlanes: nil, pixelsWide: width, pixelsHigh: height,
    bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true,
    isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0
)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
let context = NSGraphicsContext.current!.cgContext
context.translateBy(x: 0, y: CGFloat(height))
context.scaleBy(x: 1, y: -1)

func color(_ hex: UInt32, alpha: CGFloat = 1) -> NSColor {
    NSColor(srgbRed: CGFloat((hex >> 16) & 255) / 255,
            green: CGFloat((hex >> 8) & 255) / 255,
            blue: CGFloat(hex & 255) / 255, alpha: alpha)
}

func fill(_ rect: CGRect, _ hex: UInt32) {
    context.setFillColor(color(hex).cgColor)
    context.fill(rect)
}

func text(_ value: String, x: CGFloat, y: CGFloat, size: CGFloat,
          weight: NSFont.Weight = .regular, hex: UInt32, centered: Bool = false,
          tracking: CGFloat = 0) {
    let string = NSAttributedString(string: value, attributes: [
        .font: NSFont.systemFont(ofSize: size, weight: weight),
        .foregroundColor: color(hex), .kern: tracking,
    ])
    let bounds = string.size()
    // Text draws in AppKit's unflipped coordinates.
    context.saveGState()
    context.translateBy(x: centered ? x - bounds.width / 2 : x, y: y + bounds.height)
    context.scaleBy(x: 1, y: -1)
    string.draw(at: .zero)
    context.restoreGState()
}

fill(CGRect(x: 0, y: 0, width: width, height: height), 0xf7f8f4)
fill(CGRect(x: 0, y: 0, width: width, height: 138), 0x101114)

// A quiet database motif continues the app icon in the header.
context.saveGState()
context.clip(to: CGRect(x: 0, y: 0, width: width, height: 138))
context.setStrokeColor(color(0xc8f464, alpha: 0.13).cgColor)
context.setLineWidth(1.5)
for y in [75, 108, 141] {
    context.strokeEllipse(in: CGRect(x: 474, y: y, width: 216, height: 62))
}
context.move(to: CGPoint(x: 474, y: 106))
context.addLine(to: CGPoint(x: 474, y: 172))
context.strokePath()
context.restoreGState()

fill(CGRect(x: 34, y: 29, width: 28, height: 3), 0xc8f464)
text("just-db", x: 34, y: 43, size: 34, weight: .semibold, hex: 0xf7f8f4, tracking: -1)
text("Import and export PostgreSQL & MySQL", x: 35, y: 93, size: 13, hex: 0xb1b6a9)
text("FOR macOS", x: 528, y: 32, size: 10, weight: .medium, hex: 0xc8f464, tracking: 1.5)
fill(CGRect(x: 0, y: 137, width: width, height: 1), 0xc8f464)

// Finder places the real app and Applications icons on these two positions.
text("01", x: 180, y: 176, size: 10, weight: .medium, hex: 0x81877a, centered: true, tracking: 1)
text("02", x: 480, y: 176, size: 10, weight: .medium, hex: 0x81877a, centered: true, tracking: 1)

context.setStrokeColor(color(0xa5ad99).cgColor)
context.setLineWidth(1.5)
context.setLineDash(phase: 0, lengths: [2, 6])
context.move(to: CGPoint(x: 246, y: 250))
context.addCurve(to: CGPoint(x: 409, y: 250),
                 control1: CGPoint(x: 300, y: 228), control2: CGPoint(x: 355, y: 272))
context.strokePath()
context.setLineDash(phase: 0, lengths: [])
context.setLineCap(.round)
context.move(to: CGPoint(x: 400, y: 243))
context.addLine(to: CGPoint(x: 410, y: 250))
context.addLine(to: CGPoint(x: 400, y: 257))
context.strokePath()

text("Drag just-db to Applications", x: 330, y: 346, size: 17,
     weight: .medium, hex: 0x262a22, centered: true)
text("Then open just-db from your Applications folder.", x: 330, y: 374,
     size: 12, hex: 0x71776a, centered: true)
fill(CGRect(x: 34, y: 411, width: 592, height: 1), 0xe4e7dd)
text("just-db", x: 34, y: 422, size: 9, weight: .medium, hex: 0x7f8578)
text("DATABASE IMPORT & EXPORT", x: 461, y: 422, size: 8,
     weight: .medium, hex: 0x7f8578, tracking: 1)

NSGraphicsContext.restoreGraphicsState()
let output = URL(fileURLWithPath: CommandLine.arguments[1])
try bitmap.representation(using: .png, properties: [:])!.write(to: output)
print("Created \(output.path)")
