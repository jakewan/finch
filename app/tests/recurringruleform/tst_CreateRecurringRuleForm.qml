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

    Component { id: formFactory; CreateRecurringRuleForm {} }
    Component { id: mockFactory; MockFinchClient {} }

    SignalSpy { id: createdSpy; signalName: "created" }

    property var currentForm: null
    property var currentMock: null

    function build(opts) {
        opts = opts || {}
        currentMock = mockFactory.createObject(testCase)
        var props = { client: currentMock }
        if (opts.accountId !== undefined)
            props.accountId = opts.accountId
        currentForm = formFactory.createObject(testCase, props)
        verify(currentForm !== null, "form instantiated")
        return { form: currentForm, mock: currentMock }
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
    // start date, an empty end date, day-of-month 0 and no semi-monthly days. Each arg is
    // asserted independently — an 8-arg positional call is a transpose waiting to happen.
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
}
