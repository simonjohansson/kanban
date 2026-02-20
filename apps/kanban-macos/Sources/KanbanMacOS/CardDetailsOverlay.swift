import SwiftUI

struct CardDetailsOverlay: View {
    let details: KanbanCardDetails?
    let isLoading: Bool
    let errorMessage: String?
    let onClose: (String) -> Void
    let onRetry: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            header

            if isLoading {
                ProgressView("Loading card details...")
                    .frame(maxWidth: .infinity, alignment: .leading)
            } else if let errorMessage {
                VStack(alignment: .leading, spacing: 10) {
                    Text(errorMessage)
                        .foregroundStyle(.red)
                    Button("Retry") {
                        onRetry()
                    }
                    .buttonStyle(.borderedProminent)
                }
            } else if let details {
                ScrollView {
                    VStack(alignment: .leading, spacing: 14) {
                        section(title: "Branch") {
                            Text(details.branch?.trimmingCharacters(in: .whitespacesAndNewlines).nonEmpty ?? "No branch")
                                .font(.system(.body, design: .monospaced))
                        }
                        section(title: "Description") {
                            if details.description.isEmpty {
                                Text("No description")
                                    .foregroundStyle(.secondary)
                            } else {
                                ForEach(Array(details.description.enumerated()), id: \.offset) { _, event in
                                    detailItem(event.body)
                                }
                            }
                        }
                        section(title: "Comments") {
                            if details.comments.isEmpty {
                                Text("No comments")
                                    .foregroundStyle(.secondary)
                            } else {
                                ForEach(Array(details.comments.enumerated()), id: \.offset) { _, event in
                                    detailItem(event.body)
                                }
                            }
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
        .padding(16)
        .frame(width: 560, alignment: .topLeading)
        .frame(maxHeight: 600)
        .background(.regularMaterial)
        .clipShape(RoundedRectangle(cornerRadius: 12))
        .overlay(
            RoundedRectangle(cornerRadius: 12)
                .stroke(Color.gray.opacity(0.25), lineWidth: 1)
        )
        .shadow(color: Color.black.opacity(0.24), radius: 24, y: 10)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .center)
        .onExitCommand {
            onClose("escape")
        }
    }

    private var header: some View {
        HStack(alignment: .firstTextBaseline, spacing: 10) {
            Text(details?.title ?? "Card details")
                .font(.system(size: 18, weight: .semibold))
                .lineLimit(2)
            Spacer()
            Button("Close") {
                onClose("button")
            }
            .keyboardShortcut(.cancelAction)
        }
    }

    private func section<Content: View>(title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.system(size: 13, weight: .semibold))
                .foregroundStyle(.secondary)
            content()
        }
    }

    private func detailItem(_ body: String) -> some View {
        Text(body)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(8)
            .background(Color.gray.opacity(0.10))
            .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

private extension String {
    var nonEmpty: String? {
        isEmpty ? nil : self
    }
}
