import SwiftUI

struct MainSplitView: View {
    @Bindable var viewModel: ProjectsViewModel
    let zoomScale: Double
    @State private var selectedProjectID: ProjectSummary.ID?
    @State private var reviewReasonInputFocused = false
    private let sidebarProbe = SidebarStateProbe.fromEnvironment()

    var body: some View {
        let zoom = ZoomPresentation(scale: zoomScale)
        NavigationSplitView {
            List(viewModel.projects, selection: $selectedProjectID) { project in
                Text(project.name)
                    .font(.system(size: zoom.scaled(14)))
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
                ZStack {
                    GeometryReader { geometry in
                        let laneSpacing = zoom.scaled(BoardPresentation.laneSpacing)
                        let horizontalPadding = zoom.scaled(BoardPresentation.horizontalPadding)
                        let verticalPadding = zoom.scaled(BoardPresentation.verticalPadding)
                        let laneWidth = BoardPresentation.laneWidth(
                            containerWidth: geometry.size.width,
                            laneCount: KanbanLaneStatus.allCases.count,
                            laneSpacing: laneSpacing,
                            horizontalPadding: horizontalPadding
                        )

                        ScrollView(.vertical) {
                            HStack(alignment: .top, spacing: laneSpacing) {
                                ForEach(KanbanLaneStatus.allCases, id: \.rawValue) { lane in
                                    laneView(for: lane, laneWidth: laneWidth, zoom: zoom)
                                }
                            }
                            .frame(maxWidth: .infinity, alignment: .topLeading)
                            .padding(.horizontal, horizontalPadding)
                            .padding(.vertical, verticalPadding)
                        }
                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                    }

                    if viewModel.selectedCardNumber != nil {
                        CardDetailsOverlay(
                            details: viewModel.cardDetails,
                            isLoading: viewModel.isCardDetailsLoading,
                            errorMessage: viewModel.cardDetailsErrorMessage,
                            reviewActionsVisible: viewModel.cardDetails?.status == KanbanLaneStatus.review.rawValue,
                            reviewActionBusy: viewModel.isReviewTransitionInFlight,
                            reviewReasonPromptVisible: viewModel.reviewReasonPromptVisible,
                            reviewReasonTargetStatus: viewModel.reviewReasonTargetStatus,
                            reviewReasonErrorMessage: viewModel.reviewReasonErrorMessage,
                            reviewReasonInput: viewModel.reviewReasonInput,
                            onClose: { _ in
                                viewModel.closeCardDetails()
                                writeProbe()
                            },
                            onRetry: {
                                Task {
                                    await viewModel.retrySelectedCard()
                                    writeProbe()
                                }
                            },
                            onMoveReviewToTodo: {
                                Task {
                                    await viewModel.requestReviewTransition(to: KanbanLaneStatus.todo.rawValue)
                                    writeProbe()
                                }
                            },
                            onMoveReviewToDoing: {
                                Task {
                                    await viewModel.requestReviewTransition(to: KanbanLaneStatus.doing.rawValue)
                                    writeProbe()
                                }
                            },
                            onMoveReviewToDone: {
                                Task {
                                    await viewModel.requestReviewTransition(to: KanbanLaneStatus.done.rawValue)
                                    writeProbe()
                                }
                            },
                            onReviewReasonChanged: { value in
                                viewModel.reviewReasonInput = value
                                if viewModel.reviewReasonErrorMessage != nil {
                                    viewModel.reviewReasonErrorMessage = nil
                                }
                                writeProbe()
                            },
                            onSubmitReviewReason: {
                                Task {
                                    await viewModel.submitReviewReason(viewModel.reviewReasonInput)
                                    writeProbe()
                                }
                            },
                            onCancelReviewReason: {
                                viewModel.cancelReviewReasonPrompt()
                                writeProbe()
                            },
                            onReviewReasonFocusChanged: { focused in
                                reviewReasonInputFocused = focused
                                writeProbe()
                            }
                        )
                        .zIndex(2)
                    }
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
        .onChange(of: viewModel.selectedCardNumber) { _, _ in
            writeProbe()
        }
        .onChange(of: viewModel.cardDetails) { _, _ in
            writeProbe()
        }
        .onChange(of: viewModel.cardDetailsErrorMessage) { _, _ in
            writeProbe()
        }
        .onChange(of: viewModel.isCardDetailsLoading) { _, _ in
            writeProbe()
        }
        .onChange(of: viewModel.reviewReasonPromptVisible) { _, _ in
            writeProbe()
        }
        .onChange(of: viewModel.reviewReasonTargetStatus) { _, _ in
            writeProbe()
        }
        .onChange(of: viewModel.reviewReasonInput) { _, _ in
            writeProbe()
        }
        .onChange(of: viewModel.reviewReasonErrorMessage) { _, _ in
            writeProbe()
        }
        .onChange(of: viewModel.isReviewTransitionInFlight) { _, _ in
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
        .animation(.easeInOut(duration: 0.12), value: zoom.scale)
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
            if let requested = sidebarProbe.consumeSelectionRequest() {
                if let cardNumber = parseCardOpenRequest(requested) {
                    await viewModel.selectCard(number: cardNumber)
                    writeProbe()
                } else if let status = parseReviewMoveRequest(requested) {
                    await viewModel.requestReviewTransition(to: status)
                    writeProbe()
                } else if let reason = parseReviewReasonSubmitRequest(requested) {
                    await viewModel.submitReviewReason(reason)
                    writeProbe()
                } else if parseReviewReasonCancelRequest(requested) {
                    viewModel.cancelReviewReasonPrompt()
                    writeProbe()
                } else if parseCardCloseRequest(requested) {
                    viewModel.closeCardDetails()
                    writeProbe()
                } else if let slug = parseProjectSelectionRequest(requested),
                          viewModel.projects.contains(where: { $0.slug == slug }) {
                    selectedProjectID = slug
                } else if viewModel.projects.contains(where: { $0.slug == requested }) {
                    selectedProjectID = requested
                }
            }
            try? await Task.sleep(nanoseconds: 100_000_000)
        }
    }

    @ViewBuilder
    private func laneView(for lane: KanbanLaneStatus, laneWidth: Double, zoom: ZoomPresentation) -> some View {
        let cards = viewModel.cards(for: lane)
        VStack(alignment: .leading, spacing: 8) {
            Text(lane.rawValue)
                .font(.system(size: zoom.scaled(20), weight: .semibold))
            if cards.isEmpty {
                Text("No cards")
                    .font(.system(size: zoom.scaled(13)))
                    .foregroundStyle(.secondary)
            } else {
                ForEach(cards) { card in
                    VStack(alignment: .leading, spacing: zoom.scaled(4)) {
                        Text(card.title)
                            .font(.system(size: zoom.scaled(13), weight: .medium))
                            .foregroundStyle(BoardPresentation.cardTitleColor)
                            .frame(maxWidth: .infinity, alignment: .leading)
                        if let branch = card.branch?.trimmingCharacters(in: .whitespacesAndNewlines), !branch.isEmpty {
                            Text(branch)
                                .font(.system(size: zoom.scaled(11), design: .monospaced))
                                .foregroundStyle(.secondary)
                                .frame(maxWidth: .infinity, alignment: .leading)
                        }
                    }
                    .padding(.horizontal, zoom.scaled(10))
                    .padding(.vertical, zoom.scaled(8))
                    .background(BoardPresentation.cardBackgroundColor)
                    .overlay(
                        RoundedRectangle(cornerRadius: 8)
                            .stroke(Color.gray.opacity(0.25), lineWidth: 1)
                    )
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                    .onTapGesture {
                        Task {
                            await viewModel.selectCard(number: card.number)
                            writeProbe()
                        }
                    }
                }
            }
        }
        .padding(zoom.scaled(10))
        .frame(width: laneWidth, alignment: .topLeading)
        .frame(maxHeight: .infinity, alignment: .topLeading)
        .background(Color.gray.opacity(0.10))
        .clipShape(RoundedRectangle(cornerRadius: 10))
    }

    private func writeProbe() {
        sidebarProbe.write(
            projects: viewModel.projects,
            selectedProjectSlug: selectedProjectID,
            cardsByStatus: viewModel.cardsByStatusMap(),
            cardsByStatusDetailed: viewModel.cardsByStatusDetailedMap(),
            cardDetailsVisible: viewModel.selectedCardNumber != nil,
            cardDetails: viewModel.cardDetails.map {
                SidebarCardDetailsStateProbe(
                    title: $0.title,
                    branch: $0.branch,
                    descriptionBodies: $0.description.map(\.body),
                    commentBodies: $0.comments.map(\.body)
                )
            },
            cardDetailsError: viewModel.cardDetailsErrorMessage,
            reviewReasonPromptVisible: viewModel.reviewReasonPromptVisible,
            reviewReasonTargetStatus: viewModel.reviewReasonTargetStatus,
            reviewReasonError: viewModel.reviewReasonErrorMessage,
            reviewReasonInputFocused: reviewReasonInputFocused
        )
    }

    private func parseProjectSelectionRequest(_ raw: String) -> String? {
        let prefix = "project:"
        guard raw.hasPrefix(prefix) else {
            return nil
        }
        let value = String(raw.dropFirst(prefix.count))
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return value.isEmpty ? nil : value
    }

    private func parseCardOpenRequest(_ raw: String) -> Int? {
        let prefix = "card:"
        guard raw.hasPrefix(prefix) else {
            return nil
        }
        let rawNumber = String(raw.dropFirst(prefix.count))
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard let number = Int(rawNumber), number > 0 else {
            return nil
        }
        return number
    }

    private func parseCardCloseRequest(_ raw: String) -> Bool {
        raw.hasPrefix("card-close:")
    }

    private func parseReviewMoveRequest(_ raw: String) -> String? {
        let prefix = "card-review-move:"
        guard raw.hasPrefix(prefix) else {
            return nil
        }
        let value = String(raw.dropFirst(prefix.count))
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard value == KanbanLaneStatus.todo.rawValue
            || value == KanbanLaneStatus.doing.rawValue
            || value == KanbanLaneStatus.done.rawValue else {
            return nil
        }
        return value
    }

    private func parseReviewReasonSubmitRequest(_ raw: String) -> String? {
        let prefix = "card-review-reason-submit:"
        guard raw.hasPrefix(prefix) else {
            return nil
        }
        return String(raw.dropFirst(prefix.count))
    }

    private func parseReviewReasonCancelRequest(_ raw: String) -> Bool {
        raw.hasPrefix("card-review-reason-cancel:")
    }
}
