import Foundation
import CoreGraphics

let rows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
for row in rows {
    let owner = row[kCGWindowOwnerName as String] as? String ?? ""
    guard owner.lowercased().contains("mygo-player") || owner.lowercased().contains("mygo player") else { continue }
    let result: [String: Any] = [
        "id": row[kCGWindowNumber as String] ?? 0,
        "pid": row[kCGWindowOwnerPID as String] ?? 0,
        "owner": owner,
        "title": row[kCGWindowName as String] ?? "",
        "bounds": row[kCGWindowBounds as String] ?? [:],
        "layer": row[kCGWindowLayer as String] ?? 0
    ]
    let data = try JSONSerialization.data(withJSONObject: result, options: [.sortedKeys])
    print(String(data: data, encoding: .utf8)!)
}
