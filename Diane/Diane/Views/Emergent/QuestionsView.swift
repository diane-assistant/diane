import SwiftUI

struct QuestionsView: View {
    @ObservedObject var viewModel: QuestionsViewModel
    @State private var selectedQuestionId: String?

    var body: some View {
        MasterDetailView(
            master: {
                VStack(spacing: 0) {
                    MasterListHeader(
                        title: "Pending Questions",
                        trailingIcon: "questionmark.circle",
                        trailingTooltip: "Agents may ask for input when they encounter ambiguity or need confirmation to proceed."
                    )

                    if viewModel.isLoading && viewModel.questions.isEmpty {
                        Spacer()
                        ProgressView()
                        Spacer()
                    } else if viewModel.questions.isEmpty {
                        Spacer()
                        Text("No pending questions")
                            .font(.subheadline)
                            .foregroundColor(.secondary)
                            .padding(.top, 16)
                        Spacer()
                    } else {
                        ScrollView {
                            LazyVStack(spacing: 0) {
                                ForEach(viewModel.questions) { question in
                                    QuestionRow(question: question, isSelected: selectedQuestionId == question.id)
                                        .onTapGesture {
                                            selectedQuestionId = question.id
                                        }
                                    Divider()
                                }
                            }
                        }
                    }
                }
            },
            detail: {
                if let id = selectedQuestionId, let question = viewModel.questions.first(where: { $0.id == id }) {
                    QuestionDetailView(
                        question: question,
                        viewModel: viewModel
                    )
                } else {
                    VStack {
                        Spacer()
                        Text("Select a question to answer")
                            .font(.headline)
                            .foregroundColor(.secondary)
                        Spacer()
                    }
                }
            }
        )
        .onAppear {
            if selectedQuestionId == nil {
                selectedQuestionId = viewModel.questions.first?.id
            }
        }
        .onChange(of: viewModel.questions.map(\AgentQuestion.id)) { newIds in
            if let current = selectedQuestionId, !newIds.contains(current) {
                selectedQuestionId = newIds.first
            }
        }
    }
}

struct QuestionRow: View {
    let question: AgentQuestion
    let isSelected: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(question.question)
                .font(.body)
                .lineLimit(2)
                .foregroundColor(isSelected ? .white : .primary)
            Text("Agent: \(question.agentId)")
                .font(.caption)
                .foregroundColor(isSelected ? .white.opacity(0.8) : .secondary)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(isSelected ? Color.accentColor : Color.clear)
        .contentShape(Rectangle())
    }
}

struct QuestionDetailView: View {
    let question: AgentQuestion
    @ObservedObject var viewModel: QuestionsViewModel

    @State private var textResponse: String = ""
    @State private var selectedOptionValue: String? = nil
    @State private var isSubmitting: Bool = false

    var isValid: Bool {
        if question.hasOptions {
            return selectedOptionValue != nil
        } else {
            return !textResponse.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Text("Agent Question")
                    .font(.title2)
                    .fontWeight(.bold)
                Spacer()
                Text(question.createdAt, style: .time)
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
            .padding()

            Divider()

            ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                    DetailSection(title: "Question") {
                        Text(question.question)
                            .font(.body)
                            .fixedSize(horizontal: false, vertical: true)
                    }

                    if question.hasOptions {
                        DetailSection(title: "Options") {
                            VStack(alignment: .leading, spacing: 12) {
                                ForEach(question.options) { option in
                                    Button(action: {
                                        selectedOptionValue = option.value
                                    }) {
                                        HStack(alignment: .top, spacing: 12) {
                                            Image(systemName: selectedOptionValue == option.value ? "largecircle.fill.circle" : "circle")
                                                .foregroundColor(selectedOptionValue == option.value ? .accentColor : .secondary)
                                                .font(.system(size: 18))
                                            
                                            VStack(alignment: .leading, spacing: 4) {
                                                Text(option.label)
                                                    .font(.body)
                                                    .foregroundColor(.primary)
                                                if !option.description.isEmpty {
                                                    Text(option.description)
                                                        .font(.caption)
                                                        .foregroundColor(.secondary)
                                                        .fixedSize(horizontal: false, vertical: true)
                                                }
                                            }
                                        }
                                        .padding(.vertical, 4)
                                    }
                                    .buttonStyle(.plain)
                                }
                            }
                        }
                    } else {
                        DetailSection(title: "Your Answer") {
                            TextEditor(text: $textResponse)
                                .font(.body)
                                .frame(minHeight: 100)
                                .padding(8)
                                .overlay(
                                    RoundedRectangle(cornerRadius: 8)
                                        .stroke(Color.secondary.opacity(0.2), lineWidth: 1)
                                )
                        }
                    }

                    VStack(alignment: .leading, spacing: 8) {
                        Button(action: submit) {
                            if isSubmitting {
                                ProgressView().controlSize(.small)
                                    .frame(maxWidth: .infinity)
                            } else {
                                Text("Submit Answer")
                                    .fontWeight(.medium)
                                    .frame(maxWidth: .infinity)
                            }
                        }
                        .buttonStyle(.borderedProminent)
                        .controlSize(.large)
                        .disabled(!isValid || isSubmitting)

                        if let err = viewModel.error {
                            Text(err)
                                .font(.caption)
                                .foregroundColor(.red)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                    .padding(.top, 8)
                }
                .padding()
            }
        }
        .onChange(of: question.id) { _ in
            // Reset state when switching questions
            textResponse = ""
            selectedOptionValue = nil
            viewModel.error = nil
        }
    }

    private func submit() {
        guard isValid else { return }
        
        let response = question.hasOptions ? (selectedOptionValue ?? "") : textResponse.trimmingCharacters(in: .whitespacesAndNewlines)
        
        isSubmitting = true
        Task {
            do {
                try await viewModel.respond(to: question.id, response: response)
            } catch {
                // error is already handled and published by the viewmodel
            }
            isSubmitting = false
        }
    }
}
