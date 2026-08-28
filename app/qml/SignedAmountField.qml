import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "txformat.js" as TxFormat

// Shared signed-amount input: a magnitude field whose accepted shape is enforced by one pattern at
// both the keystroke validator and the submit predicate (so it forbids a sign either way), an
// Expense/Income toggle as the sole sign source, and a live signed-amount preview. Extracted
// from the transaction and recurring-rule forms so the 64-bit-overflow-safe cents handling
// lives in one place. The host reads `signedCents`/`amountValid` and reacts to `edited()`.
Item {
    id: control

    // Public, test-facing surface — hosts and specs drive the control through these.
    property alias amountText: amountField.text
    // Expense (true, default) vs Income. The magnitude field cannot carry a sign, so this
    // toggle alone decides whether cents are negative or positive.
    property bool expense: true
    // Hosts whose value has no user-chosen direction hide the toggle — a recurring transfer's
    // direction comes from its source/target pair, not from the author. Kept as a plain
    // property (not an alias to the row's `visible`) because an effective-visibility read is
    // unreliable offscreen, so this is what a spec asserts.
    property bool signSelectorVisible: true

    // The one accepted-text pattern, enforced in two places: the field's validator (which gates
    // typed keystrokes) and amountValid (which gates everything else). A property write bypasses
    // the validator entirely — every spec and every host sets amountText directly — so a shape
    // the predicate does not also refuse is accepted in practice.
    //
    // Two properties of this literal are correctness-critical, not stylistic:
    //   - No `g` flag. A global JS regex carries lastIndex across .test() calls, so consecutive
    //     evaluations of the same text would alternate true/false. Only the predicate side would
    //     be affected, which would present as an inexplicable binding bug.
    //   - The ^…$ anchors. RegularExpressionValidator matches against the whole input, but JS
    //     .test() substring-searches unless anchored — so dropping them would have the predicate
    //     accept "12.34abc" while the validator rejected it, reopening this exact hole.
    //
    // Locale-independent: always ".", never a comma decimal (parseFloat truncates "19,99" to
    // 19), and no leading "-" so the Expense/Income toggle stays the sole sign source. At most 2
    // decimals, so no entry can carry precision that rounding would silently discard. The 12
    // integer digits cap the amount just under $1 trillion, which keeps cents at or below
    // ~1.0e14 — comfortably inside the 2^53 ceiling where a JS number still represents integers
    // exactly on its way to a 64-bit sink. A trailing bare "." stays legal so typing one is not
    // rejected mid-entry.
    readonly property var amountPattern: /^\d{1,12}(\.\d{0,2})?$/

    // parseFloat reads the magnitude locale-cleanly because the pattern above forbids a sign and
    // a comma decimal. NaN when blank or non-numeric.
    readonly property real magnitude: parseFloat(amountField.text)
    // Reject 0: a $0 entry is meaningless. Core and the daemon now reject one too, so this is the
    // first gate rather than the only one — it exists to answer in the form instead of spending a
    // round trip. No separate isNaN check — NaN > 0 is false — and no separate magnitude bound,
    // because the pattern's digit cap *is* the bound. Two numeric bounds would be two different
    // limits wearing one name.
    //
    // That digit cap and core.MaxAmountCents are the same number, reached by two arguments: this
    // field from the 2^53 seam where a JS number stops representing cents exactly, the domain from
    // the int64 headroom its projection sums need. Keep them equal; if they ever must diverge, the
    // domain governs and this side moves.
    readonly property bool amountValid: amountPattern.test(amountField.text) && magnitude > 0
    // Math.round avoids float drift (12.34 * 100 -> 1233.9999…).
    // Typed var, not int: cents cross into a 64-bit qlonglong and a QML int is 32-bit — an int
    // would overflow above ~$21.5M. A JS number marshals to qlonglong exactly (< 2^53).
    //
    // The unsigned magnitude is its own property rather than something a host derives by
    // negating signedCents: a host that needs a positive value (a transfer, whose amount is a
    // magnitude the daemon rejects unless positive) would otherwise send a negative one
    // whenever Income was the standing choice. Sign-free by construction.
    readonly property var unsignedCents: amountValid ? Math.round(magnitude * 100) : 0
    readonly property var signedCents: amountValid ? unsignedCents * (expense ? -1 : 1) : 0
    readonly property string previewText: amountValid ? TxFormat.formatAmount(signedCents) : ""

    // Emitted on any edit (magnitude or sign) so the host can clear a stale error message.
    signal edited()

    implicitWidth: layout.implicitWidth
    implicitHeight: layout.implicitHeight

    ColumnLayout {
        id: layout
        anchors.fill: parent
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            spacing: 12

            TextField {
                id: amountField
                Layout.fillWidth: true
                placeholderText: "0.00"
                inputMethodHints: Qt.ImhFormattedNumbersOnly
                // Shares control.amountPattern with amountValid — see there for why the shape is
                // enforced in both places and which parts of the literal are load-bearing.
                validator: RegularExpressionValidator {
                    regularExpression: control.amountPattern
                }
                onTextChanged: control.edited()
            }

            // Live signed-amount preview: the sign is visible before submit.
            Label {
                text: control.previewText
                color: control.expense ? "#c0392b" : "#27ae60"
                visible: control.previewText.length > 0
            }
        }

        // The magnitude field cannot carry a sign, so this toggle is the only way the user
        // expresses direction — a control that can't express the wrong value.
        RowLayout {
            spacing: 12
            visible: control.signSelectorVisible
            ButtonGroup { id: signGroup }
            RadioButton {
                text: "Expense"
                ButtonGroup.group: signGroup
                checked: control.expense
                onClicked: { control.expense = true; control.edited() }
            }
            RadioButton {
                text: "Income"
                ButtonGroup.group: signGroup
                onClicked: { control.expense = false; control.edited() }
            }
        }
    }
}
