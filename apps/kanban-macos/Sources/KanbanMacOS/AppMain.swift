import AppKit
import SwiftUI

@main
struct KanbanMacOSApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @State private var viewModel = KanbanMacOSApp.makeViewModel()
    @State private var zoomController = ZoomController()

    var body: some Scene {
        WindowGroup {
            MainSplitView(viewModel: viewModel, zoomScale: zoomController.scale)
                .frame(minWidth: 900, minHeight: 560)
        }
        .commands {
            CommandMenu("View") {
                Button("Zoom In") {
                    zoomController.zoomIn()
                }
                .keyboardShortcut("=", modifiers: [.command])

                Button("Zoom Out") {
                    zoomController.zoomOut()
                }
                .keyboardShortcut("-", modifiers: [.command])

                Button("Actual Size") {
                    zoomController.reset()
                }
                .keyboardShortcut("0", modifiers: [.command])
            }
        }
    }

    @MainActor
    private static func makeViewModel() -> ProjectsViewModel {
        let home = FileManager.default.homeDirectoryForCurrentUser.path(percentEncoded: false)
        let config = (try? AppConfig.load(homeDirectory: home)) ?? .fallback

        let apiClient: any ProjectsAPIClient
        if let client = try? OpenAPIProjectsClient(serverURL: config.serverURL) {
            apiClient = client
        } else {
            apiClient = FallbackProjectsClient()
        }

        let stream = WebSocketProjectEventStream(serverURL: config.serverURL)
        let store = ProjectsStore(apiClient: apiClient, eventStream: stream)
        return ProjectsViewModel(store: store)
    }
}

private struct FallbackProjectsClient: ProjectsAPIClient {
    func listProjects() async throws -> [ProjectSummary] { [] }
    func listCards(projectSlug _: String) async throws -> [KanbanCardSummary] { [] }
    func getCard(projectSlug _: String, number _: Int) async throws -> KanbanCardDetails {
        throw URLError(.badServerResponse)
    }
}

private final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        if ProcessInfo.processInfo.environment["KANBAN_E2E_DISABLE_ACTIVATION"] == "1" {
            return
        }
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)
    }
}
