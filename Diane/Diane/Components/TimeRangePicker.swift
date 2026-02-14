import SwiftUI

/// A segmented picker for selecting a time range.
///
/// Generic over any `CaseIterable & Hashable & RawRepresentable` type whose
/// raw value is `String`, so it works with any enum whose cases are
/// display‐ready labels (e.g. "24 Hours", "7 Days").
///
/// Usage:
///
///     TimeRangePicker(selection: $viewModel.selectedTimeRange)
///
struct TimeRangePicker<T: CaseIterable & Hashable & RawRepresentable>: View
    where T.RawValue == String, T.AllCases: RandomAccessCollection
{
    @Binding var selection: T

    var body: some View {
        Picker(selection: $selection) {
            ForEach(T.allCases, id: \.self) { range in
                Text(range.rawValue).tag(range)
            }
        } label: {
            EmptyView()
        }
        .labelsHidden()
        .pickerStyle(.segmented)
    }
}
