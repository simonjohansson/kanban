import SwiftUI

struct MainSplitView: View {
    @Bindable var viewModel: ProjectsViewModel
    @State private var selectedProjectID: ProjectSummary.ID?
    private let sidebarProbe = SidebarStateProbe.fromEnvironment()

    var body: some View {
        NavigationSplitView {
            List(viewModel.projects, selection: $selectedProjectID) { project in
                Text(project.name)
                    .help(tooltip(for: project))
            }
            .navigationTitle("Projects")
        } detail: {
            Color.clear
                .overlay {
                    Text("No project selected")
                        .foregroundStyle(.secondary)
                }
        }
        .task {
            sidebarProbe.write(projects: viewModel.projects, selectedProjectSlug: selectedProjectID)
            await viewModel.load()
        }
        .onChange(of: viewModel.projects) { _, latest in
            sidebarProbe.write(projects: latest, selectedProjectSlug: selectedProjectID)
        }
        .onChange(of: selectedProjectID) { _, latest in
            sidebarProbe.write(projects: viewModel.projects, selectedProjectSlug: latest)
        }
        .task {
            await processSelectionRequests()
        }
        .alert(
            "Error",
            isPresented: Binding(
                get: { viewModel.alertMessage != nil },
                set: { newValue in
                    if !newValue {
                        viewModel.alertMessage = nil
                    }
                }
            )
        ) {
            Button("OK", role: .cancel) {
                viewModel.alertMessage = nil
            }
        } message: {
            Text(viewModel.alertMessage ?? "Unknown error")
        }
    }

    private func tooltip(for project: ProjectSummary) -> String {
        let local = project.localPath?.trimmingCharacters(in: .whitespacesAndNewlines)
        let remote = project.remoteURL?.trimmingCharacters(in: .whitespacesAndNewlines)
        var lines: [String] = []
        if let local, !local.isEmpty {
            lines.append("Local: \(local)")
        }
        if let remote, !remote.isEmpty {
            lines.append("Remote: \(remote)")
        }
        if lines.isEmpty {
            return project.slug
        }
        return lines.joined(separator: "\n")
    }

    private func processSelectionRequests() async {
        guard sidebarProbe.selectionInputURL != nil else {
            return
        }
        while !Task.isCancelled {
            if let requested = sidebarProbe.consumeSelectionRequest(),
               viewModel.projects.contains(where: { $0.slug == requested }) {
                selectedProjectID = requested
            }
            try? await Task.sleep(nanoseconds: 100_000_000)
        }
    }
}
