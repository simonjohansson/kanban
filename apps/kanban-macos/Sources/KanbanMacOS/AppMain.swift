import AppKit
import SwiftUI

@main
struct KanbanMacOSApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @State private var viewModel = KanbanMacOSApp.makeViewModel()

    var body: some Scene {
        WindowGroup {
            MainSplitView(viewModel: viewModel)
                .frame(minWidth: 900, minHeight: 560)
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
