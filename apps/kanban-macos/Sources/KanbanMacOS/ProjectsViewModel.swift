import Foundation
import Observation

@MainActor
@Observable
public final class ProjectsViewModel {
    public private(set) var projects: [ProjectSummary] = []
    public var alertMessage: String?

    private let store: any ProjectsStoreProtocol
    private var watchTask: Task<Void, Never>?

    public init(store: any ProjectsStoreProtocol) {
        self.store = store
    }

    public func load() async {
        do {
            projects = try await store.initialLoad()
            watchTask?.cancel()
            watchTask = Task { [store] in
                do {
                    for try await update in store.startWatching() {
                        await MainActor.run {
                            self.projects = update
                        }
                    }
                } catch {
                    await MainActor.run {
                        self.alertMessage = "Project stream failed: \(error.localizedDescription)"
                    }
                }
            }
        } catch {
            alertMessage = "Failed to load projects: \(error.localizedDescription)"
        }
    }
}
