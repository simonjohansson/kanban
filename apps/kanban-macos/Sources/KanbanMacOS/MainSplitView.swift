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
            if selectedProjectID == nil {
                Color.clear
                    .overlay {
                        Text("No project selected")
                            .foregroundStyle(.secondary)
                    }
            } else {
                GeometryReader { geometry in
                    let laneWidth = BoardPresentation.laneWidth(
                        containerWidth: geometry.size.width,
                        laneCount: KanbanLaneStatus.allCases.count
                    )

                    ScrollView(.vertical) {
                        HStack(alignment: .top, spacing: BoardPresentation.laneSpacing) {
                            ForEach(KanbanLaneStatus.allCases, id: \.rawValue) { lane in
                                laneView(for: lane, laneWidth: laneWidth)
                            }
                        }
                        .frame(maxWidth: .infinity, alignment: .topLeading)
                        .padding(.horizontal, BoardPresentation.horizontalPadding)
                        .padding(.vertical, BoardPresentation.verticalPadding)
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                }
            }
        }
        .task {
            await viewModel.load()
            selectedProjectID = viewModel.selectedProjectSlug
            writeProbe()
        }
        .onChange(of: viewModel.projects) { _, latest in
            if let selectedProjectID, !latest.contains(where: { $0.slug == selectedProjectID }) {
                self.selectedProjectID = nil
            }
            writeProbe()
        }
        .onChange(of: viewModel.cards) { _, _ in
            writeProbe()
        }
        .onChange(of: selectedProjectID) { _, latest in
            Task {
                await viewModel.selectProject(slug: latest)
                writeProbe()
            }
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

    @ViewBuilder
    private func laneView(for lane: KanbanLaneStatus, laneWidth: Double) -> some View {
        let cards = viewModel.cards(for: lane)
        VStack(alignment: .leading, spacing: 8) {
            Text(lane.rawValue)
                .font(.headline)
            if cards.isEmpty {
                Text("No cards")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            } else {
                ForEach(cards) { card in
                    Text(card.title)
                        .font(.subheadline)
                        .foregroundStyle(BoardPresentation.cardTitleColor)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.horizontal, 10)
                        .padding(.vertical, 8)
                        .background(BoardPresentation.cardBackgroundColor)
                        .overlay(
                            RoundedRectangle(cornerRadius: 8)
                                .stroke(Color.gray.opacity(0.25), lineWidth: 1)
                        )
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                }
            }
        }
        .padding(10)
        .frame(width: laneWidth, alignment: .topLeading)
        .frame(maxHeight: .infinity, alignment: .topLeading)
        .background(Color.gray.opacity(0.10))
        .clipShape(RoundedRectangle(cornerRadius: 10))
    }

    private func writeProbe() {
        sidebarProbe.write(
            projects: viewModel.projects,
            selectedProjectSlug: selectedProjectID,
            cardsByStatus: viewModel.cardsByStatusMap()
        )
    }
}
