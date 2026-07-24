import QtQuick
import QtTest
// FinchRecurringFormTest is the test-only module (see app/CMakeLists.txt) holding the real
// shipping CreateRecurringRuleForm, the txformat.js helper it imports, and the shared
// MockFinchClient double — scoped to just those files.
import FinchRecurringFormTest

TestCase {
    id: testCase
    name: "CreateRecurringRuleForm"
    when: windowShown

    // Frequency enum values (proto Frequency): Weekly=1 … Yearly=5. The ComboBox has no
    // sentinel row, so a row's index is one less than its value — picking index 3 (Monthly)
    // must submit value 4, which is what catches an index-vs-value bug.
    readonly property int idxWeekly: 0
    readonly property int idxSemiMonthly: 2
    readonly property int idxMonthly: 3

    // Three accounts, and the transfer specs deliberately source from the MIDDLE one. With a
    // sentinel row at index 0 and the source filtered out, a source at index 0 makes the
    // sentinel's +1 shift cancel the filter's -1 shift exactly — filtered row i would then be
    // accounts[i] at every row, and a spec asserting the submitted id could not tell a correct
    // currentValue read from an accounts[currentIndex] lookup. Sourcing from a2 breaks that
    // coincidence: filtered row 1 is a1, while accounts[1] is a2.
    readonly property var sampleAccounts: [
        { id: "a1", name: "Checking" },
        { id: "a2", name: "Savings" },
        { id: "a3", name: "Credit" }
    ]
    // Index 0 is the "Not a transfer" sentinel; 1 and 2 are the two non-source accounts.
    readonly property int idxNoTransfer: 0
    readonly property int idxFirstTarget: 1
    readonly property int idxLastTarget: 2

    Component { id: formFactory; CreateRecurringRuleForm {} }
    Component { id: mockFactory; MockFinchClient {} }

    SignalSpy { id: createdSpy; signalName: "created" }

    property var currentForm: null
    property var currentMock: null

    function build(opts) {
        opts = opts || {}
        currentMock = mockFactory.createObject(testCase)
        // Accounts are set before the form is built so the destination model is populated at
        // completion, matching how the panel suite seeds its selector.
        currentMock.accounts = (opts.accounts !== undefined) ? opts.accounts : sampleAccounts
        var props = { client: currentMock }
        if (opts.accountId !== undefined)
            props.accountId = opts.accountId
        currentForm = formFactory.createObject(testCase, props)
        verify(currentForm !== null, "form instantiated")
        return { form: currentForm, mock: currentMock }
    }

    // A valid weekly rule sourced from the middle account, ready for a destination choice.
    function fillValidTransferFrom(ctx) {
        ctx.form.nameText = "To savings"
        ctx.form.amountText = "500"
        ctx.form.frequencyIndex = idxWeekly
    }

    function cleanup() {
        createdSpy.target = null
        if (currentForm) { currentForm.destroy(); currentForm = null }
        if (currentMock) { currentMock.destroy(); currentMock = null }
    }

    // Fills a valid Weekly expense: account injected, name + magnitude + a frequency set. The
    // start date defaults to today, so a minimal weekly rule needs no date input.
    function fillValidWeekly(ctx) {
        ctx.form.accountId = "a1"
        ctx.form.nameText = "Gym"
        ctx.form.amountText = "40"
        ctx.form.frequencyIndex = idxWeekly
    }

    function test_loadsAndStartsInvalid() {
        var ctx = build()
        compare(ctx.form.canSubmit, false)
    }

    function test_disabledWithoutFrequency() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Gym"
        ctx.form.amountText = "40"
        // No frequency chosen yet.
        compare(ctx.form.canSubmit, false)
    }

    function test_enabledWeeklyMinimal() {
        var ctx = build()
        fillValidWeekly(ctx)
        compare(ctx.form.canSubmit, true)
    }

    function test_disabledWhenNoAccount() {
        var ctx = build()
        ctx.form.nameText = "Gym"
        ctx.form.amountText = "40"
        ctx.form.frequencyIndex = idxWeekly
        compare(ctx.form.accountId, "")
        compare(ctx.form.canSubmit, false)
    }

    function test_disabledWhenNameBlank() {
        var ctx = build({ accountId: "a1" })
        ctx.form.amountText = "40"
        ctx.form.frequencyIndex = idxWeekly
        ctx.form.nameText = "   "
        compare(ctx.form.canSubmit, false)
    }

    function test_disabledWhenAmountZero() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Gym"
        ctx.form.frequencyIndex = idxWeekly
        ctx.form.amountText = "0"
        compare(ctx.form.canSubmit, false)
    }

    // A weekly rule sends signed cents, the trimmed name, the account, frequency value 1, the
    // start date, an empty end date, day-of-month 0, no semi-monthly days, and — with no
    // destination chosen — no transfer. Each field is asserted independently.
    //
    // The empty target matters beyond completeness: core persists a non-empty
    // transfer_target_account_id regardless of is_transfer, so a form that sent the picker's
    // value unconditionally would write phantom-target rules.
    function test_submitSendsWeeklyArgs() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "  Gym  "
        ctx.form.amountText = "40"
        ctx.form.expense = true
        ctx.form.frequencyIndex = idxWeekly
        ctx.form.startDateText = "2025-06-01"
        ctx.form.submit()
        compare(ctx.mock.createRecurringRuleCallCount, 1)
        compare(ctx.mock.lastCreateAccountId, "a1")
        compare(ctx.mock.lastCreateName, "Gym")
        compare(ctx.mock.lastCreateAmount, -4000)
        compare(ctx.mock.lastCreateFrequency, 1)
        compare(ctx.mock.lastCreateStartDate, "2025-06-01")
        compare(ctx.mock.lastCreateEndDate, "")
        compare(ctx.mock.lastCreateDayOfMonth, 0)
        compare(ctx.mock.lastCreateSemiMonthlyDays.length, 0)
        compare(ctx.mock.lastCreateIsTransfer, false)
        compare(ctx.mock.lastCreateTransferTargetAccountId, "")
    }

    // The params object removes the positional-transpose hazard but adds a silent one: a
    // mistyped key arrives absent rather than erroring. This pins the exact key set the C++
    // reader unpacks, which no other spec can reach across the QML/C++ seam.
    function test_submitSendsExpectedParamKeys() {
        var ctx = build({ accountId: "a1" })
        fillValidWeekly(ctx)
        ctx.form.submit()
        compare(ctx.mock.lastCreateKeys.join(","),
                "accountId,amount,dayOfMonth,endDate,frequency,isTransfer,name,"
                + "semiMonthlyDays,startDate,transferTargetAccountId")
    }

    function test_incomeIsPositive() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Paycheck"
        ctx.form.amountText = "2000"
        ctx.form.expense = false
        ctx.form.frequencyIndex = idxWeekly
        ctx.form.submit()
        compare(ctx.mock.lastCreateAmount, 200000)
    }

    function test_signIntegrity() {
        var ctx = build({ accountId: "a1" })
        ctx.form.amountText = "9.99"
        ctx.form.expense = true
        verify(ctx.form.signedCents < 0)
        ctx.form.expense = false
        verify(ctx.form.signedCents > 0)
    }

    // Above the 32-bit int ceiling: signedCents is a JS number (not a QML int), so ~$30M does
    // not overflow on the way to the client's 64-bit qlonglong amount.
    function test_largeAmountDoesNotOverflow() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Mortgage payoff"
        ctx.form.amountText = "30000000"
        ctx.form.expense = true
        ctx.form.frequencyIndex = idxWeekly
        compare(ctx.form.signedCents, -3000000000)
        ctx.form.submit()
        compare(ctx.mock.lastCreateAmount, -3000000000)
    }

    // Monthly is index 3 but enum value 4. Submitting the value (not the index) is what this
    // asserts — with no sentinel row, index != value, so a currentIndex-instead-of-value bug
    // would send 3 and fail here.
    function test_monthlySubmitsFrequencyValueNotIndex() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Rent"
        ctx.form.amountText = "1500"
        ctx.form.frequencyIndex = idxMonthly
        ctx.form.dayOfMonthText = "15"
        ctx.form.submit()
        compare(ctx.mock.lastCreateFrequency, 4)
        compare(ctx.mock.lastCreateDayOfMonth, 15)
        compare(ctx.mock.lastCreateSemiMonthlyDays.length, 0)
    }

    function test_monthlyRequiresDayInRange() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Rent"
        ctx.form.amountText = "1500"
        ctx.form.frequencyIndex = idxMonthly
        ctx.form.dayOfMonthText = ""          // missing
        compare(ctx.form.canSubmit, false)
        ctx.form.dayOfMonthText = "40"        // out of range
        compare(ctx.form.canSubmit, false)
        ctx.form.dayOfMonthText = "15"        // valid
        compare(ctx.form.canSubmit, true)
    }

    function test_semiMonthlyRequiresTwoDaysInRange() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Paycheck"
        ctx.form.amountText = "2000"
        ctx.form.frequencyIndex = idxSemiMonthly
        ctx.form.semiDay1Text = "1"
        ctx.form.semiDay2Text = ""            // second missing
        compare(ctx.form.canSubmit, false)
        ctx.form.semiDay2Text = "40"          // out of range
        compare(ctx.form.canSubmit, false)
        ctx.form.semiDay2Text = "15"          // both valid
        compare(ctx.form.canSubmit, true)
    }

    function test_semiMonthlySubmitsBothDays() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Paycheck"
        ctx.form.amountText = "2000"
        ctx.form.frequencyIndex = idxSemiMonthly
        ctx.form.semiDay1Text = "1"
        ctx.form.semiDay2Text = "15"
        ctx.form.submit()
        compare(ctx.mock.lastCreateFrequency, 3)
        compare(ctx.mock.lastCreateDayOfMonth, 0)
        compare(ctx.mock.lastCreateSemiMonthlyDays.length, 2)
        compare(ctx.mock.lastCreateSemiMonthlyDays[0], 1)
        compare(ctx.mock.lastCreateSemiMonthlyDays[1], 15)
    }

    // An end date before the start date creates a rule that projects zero occurrences (the
    // daemon and core do not validate ordering), so the form must block it.
    function test_endDateBeforeStartDisables() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Gym"
        ctx.form.amountText = "40"
        ctx.form.frequencyIndex = idxWeekly
        ctx.form.startDateText = "2025-06-01"
        ctx.form.endDateText = "2025-05-01"   // before start
        compare(ctx.form.canSubmit, false)
        ctx.form.endDateText = "2025-07-01"   // after start
        compare(ctx.form.canSubmit, true)
        ctx.form.endDateText = ""             // open-ended
        compare(ctx.form.canSubmit, true)
    }

    function test_submitIgnoredWhenInvalid() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "   "
        ctx.form.amountText = "40"
        ctx.form.frequencyIndex = idxWeekly
        ctx.form.submit()
        compare(ctx.mock.createRecurringRuleCallCount, 0)
    }

    function test_successClearsFieldsAndSignalsCreated() {
        var ctx = build({ accountId: "a1" })
        createdSpy.target = ctx.form
        createdSpy.clear()
        fillValidWeekly(ctx)
        ctx.form.submit()
        ctx.mock.succeedCreateRecurringRule("rule-1")
        compare(createdSpy.count, 1)
        compare(ctx.form.nameText, "")
        compare(ctx.form.amountText, "")
        createdSpy.target = null
    }

    function test_failureShowsErrorMessage() {
        var ctx = build({ accountId: "a1" })
        fillValidWeekly(ctx)
        ctx.form.submit()
        ctx.mock.failCreateRecurringRule("day_of_month must be 1-31")
        compare(ctx.form.errorText, "day_of_month must be 1-31")
    }

    function test_disabledWhileCreateInProgress() {
        var ctx = build({ accountId: "a1" })
        fillValidWeekly(ctx)
        verify(ctx.form.canSubmit)
        ctx.form.submit()
        compare(ctx.mock.createRecurringRuleInProgress, true)
        compare(ctx.form.canSubmit, false)
    }

    function test_doubleSubmitSendsOnlyOne() {
        var ctx = build({ accountId: "a1" })
        fillValidWeekly(ctx)
        ctx.form.submit()
        ctx.form.submit()
        compare(ctx.mock.createRecurringRuleCallCount, 1)
    }

    // --- Transfer mode ---------------------------------------------------------------------

    // The destination list offers "not a transfer" as a real selectable row, not merely a
    // placeholder, so a user who picks a destination can get back out without submitting. The
    // source account is absent, which is what makes choosing it as its own target impossible
    // rather than merely invalid.
    function test_transferTargetsExcludeSourceAndOfferSentinel() {
        var ctx = build({ accountId: "a2" })
        var targets = ctx.form.transferTargets
        compare(targets.length, 3)
        compare(targets[idxNoTransfer].id, "")
        compare(targets[idxFirstTarget].id, "a1")
        compare(targets[idxLastTarget].id, "a3")
    }

    // Sourced from the middle account, filtered row 1 is a1 while accounts[1] is a2 — so this
    // asserts the form submits the row's *id* and not an index into the unfiltered list.
    function test_transferSubmitsSelectedRowIdNotIndex() {
        var ctx = build({ accountId: "a2" })
        fillValidTransferFrom(ctx)
        ctx.form.transferTargetIndex = idxFirstTarget
        ctx.form.submit()
        compare(ctx.mock.lastCreateIsTransfer, true)
        compare(ctx.mock.lastCreateTransferTargetAccountId, "a1")
        compare(ctx.mock.lastCreateAccountId, "a2")
    }

    function test_transferSubmitsLastRowId() {
        var ctx = build({ accountId: "a2" })
        fillValidTransferFrom(ctx)
        ctx.form.transferTargetIndex = idxLastTarget
        ctx.form.submit()
        compare(ctx.mock.lastCreateTransferTargetAccountId, "a3")
    }

    // A transfer's amount is a positive magnitude — direction comes from the source/target
    // pair, and both the daemon and core reject a non-positive one. Expense is the form's
    // default, so a form that passed its signed cents straight through would send -50000.
    function test_transferSendsPositiveAmountDespiteExpenseDefault() {
        var ctx = build({ accountId: "a2" })
        fillValidTransferFrom(ctx)
        ctx.form.expense = true
        ctx.form.transferTargetIndex = idxFirstTarget
        ctx.form.submit()
        compare(ctx.mock.lastCreateAmount, 50000)
    }

    // The magnitude is sign-free by construction rather than by negating the signed value —
    // negation would send a negative amount whenever Income was the standing choice.
    function test_unsignedCentsPositiveForBothSigns() {
        var ctx = build({ accountId: "a2" })
        ctx.form.amountText = "500"
        ctx.form.expense = true
        compare(ctx.form.unsignedCents, 50000)
        ctx.form.expense = false
        compare(ctx.form.unsignedCents, 50000)
    }

    // Returning to the sentinel leaves transfer mode — the round trip a placeholder-only
    // control could not express.
    function test_returningToSentinelClearsTransfer() {
        var ctx = build({ accountId: "a2" })
        fillValidTransferFrom(ctx)
        ctx.form.transferTargetIndex = idxFirstTarget
        verify(ctx.form.isTransfer)
        ctx.form.transferTargetIndex = idxNoTransfer
        compare(ctx.form.isTransfer, false)
        ctx.form.submit()
        compare(ctx.mock.lastCreateIsTransfer, false)
        compare(ctx.mock.lastCreateTransferTargetAccountId, "")
    }

    // Switching source must not leave a destination selected against the old list. Asserting
    // the *submitted* target, not just the selection, is what catches a reset that loses a race
    // with the model's own re-evaluation.
    function test_changingSourceDropsHeldDestination() {
        var ctx = build({ accountId: "a2" })
        fillValidTransferFrom(ctx)
        ctx.form.transferTargetIndex = idxFirstTarget
        ctx.form.accountId = "a3"
        compare(ctx.form.isTransfer, false)
        ctx.form.submit()
        compare(ctx.mock.lastCreateIsTransfer, false)
        compare(ctx.mock.lastCreateTransferTargetAccountId, "")
    }

    // A follow-up rule must not silently inherit transfer mode from the one just created.
    function test_successResetsDestinationToSentinel() {
        var ctx = build({ accountId: "a2" })
        fillValidTransferFrom(ctx)
        ctx.form.transferTargetIndex = idxFirstTarget
        ctx.form.submit()
        ctx.mock.succeedCreateRecurringRule("rule-1")
        compare(ctx.form.isTransfer, false)
        compare(ctx.form.transferTargetIndex, idxNoTransfer)
    }

    // Direction is implied by the source/target pair, so the Expense/Income choice is
    // meaningless in transfer mode and the control goes away. Asserted on the logical property,
    // never on `visible`, which reads false offscreen regardless.
    function test_signSelectorSuppressedInTransferMode() {
        var ctx = build({ accountId: "a2" })
        compare(ctx.form.signSelectorVisible, true)
        ctx.form.transferTargetIndex = idxFirstTarget
        compare(ctx.form.signSelectorVisible, false)
        ctx.form.transferTargetIndex = idxNoTransfer
        compare(ctx.form.signSelectorVisible, true)
    }

    // The preview must describe the leg the rule actually writes. Entering transfer mode with
    // Income standing would otherwise leave a green "+" preview above a rule that debits the
    // source — the app stating the wrong direction at the moment of authoring.
    function test_transferPreviewReadsAsDebit() {
        var ctx = build({ accountId: "a2" })
        ctx.form.amountText = "500"
        ctx.form.expense = false
        compare(ctx.form.previewText, "$500.00")
        ctx.form.transferTargetIndex = idxFirstTarget
        compare(ctx.form.previewText, "-$500.00")
    }

    // Leaving transfer mode restores the sign the user had chosen rather than silently
    // discarding it, matching the form's standing posture of preserving in-progress input.
    function test_leavingTransferModeRestoresPriorSign() {
        var ctx = build({ accountId: "a2" })
        ctx.form.amountText = "500"
        ctx.form.expense = false
        ctx.form.transferTargetIndex = idxFirstTarget
        compare(ctx.form.expense, true)
        ctx.form.transferTargetIndex = idxNoTransfer
        compare(ctx.form.expense, false)
    }

    // Past the 32-bit ceiling, and positive: a transfer magnitude must survive the same way
    // the signed path does.
    function test_largeTransferAmountStaysPositive() {
        var ctx = build({ accountId: "a2" })
        ctx.form.nameText = "Payoff sweep"
        ctx.form.amountText = "30000000"
        ctx.form.frequencyIndex = idxWeekly
        ctx.form.transferTargetIndex = idxFirstTarget
        ctx.form.submit()
        compare(ctx.mock.lastCreateAmount, 3000000000)
    }

    // A stale daemon error (e.g. a rejected target) must not survive the correction, matching
    // every other input in the form.
    function test_destinationChangeClearsError() {
        var ctx = build({ accountId: "a2" })
        fillValidTransferFrom(ctx)
        ctx.form.submit()
        ctx.mock.failCreateRecurringRule("transfer target account not found")
        compare(ctx.form.errorText, "transfer target account not found")
        ctx.form.transferTargetIndex = idxFirstTarget
        compare(ctx.form.errorText, "")
    }

    // With no source chosen the form is disabled anyway, but the list must not offer a
    // destination before a source exists.
    function test_noSourceOffersOnlySentinel() {
        var ctx = build()
        compare(ctx.form.accountId, "")
        compare(ctx.form.transferTargets.length, 1)
        compare(ctx.form.transferTargets[idxNoTransfer].id, "")
    }
}
