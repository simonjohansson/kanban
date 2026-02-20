// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "kanban-macos",
    platforms: [
        .macOS(.v14)
    ],
    products: [
        .executable(name: "KanbanMacOS", targets: ["KanbanMacOS"])
    ],
    dependencies: [
        .package(url: "https://github.com/apple/swift-openapi-generator", from: "1.8.0"),
        .package(url: "https://github.com/apple/swift-openapi-runtime", from: "1.9.0"),
        .package(url: "https://github.com/apple/swift-openapi-urlsession", from: "1.1.0"),
    ],
    targets: [
        .target(
            name: "KanbanAPI",
            dependencies: [
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
            ],
            plugins: [
                .plugin(name: "OpenAPIGenerator", package: "swift-openapi-generator"),
            ]
        ),
        .executableTarget(
            name: "KanbanMacOS",
            dependencies: [
                "KanbanAPI",
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "OpenAPIURLSession", package: "swift-openapi-urlsession"),
            ]
        ),
        .testTarget(
            name: "KanbanMacOSTests",
            dependencies: ["KanbanMacOS"]
        ),
        .testTarget(
            name: "KanbanMacOSE2ETests",
            dependencies: [
                "KanbanAPI",
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "OpenAPIURLSession", package: "swift-openapi-urlsession"),
            ]
        ),
    ]
)
