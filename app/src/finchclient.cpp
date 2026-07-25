#include "finchclient.h"

#include <QDate>
#include <QDateTime>
#include <QTime>
#include <QTimer>
#include <QVariantMap>
#include <QtConcurrent>
#include <QtCharts/QXYSeries>
#include <chrono>
#include <grpcpp/grpcpp.h>

#include <limits>

static constexpr int kMinConnectingMs = 300;

// Shared by the two params-object writers (recordTransaction, createRecurringRule). Returns the
// comma-joined keys that are absent or unreadable, empty when every key is present and valid —
// two distinct defects, and only one of them is a missing key: a misspelled *key* arrives absent,
// while a typo in a key's *value* expression arrives as a present-but-invalid variant. The second
// is the dangerous one, because each caller's conversions would turn it into a plausible value
// rather than an error.
//
// Returns the list rather than emitting, because the callers differ in exactly the two ways a
// shared helper cannot absorb: which failure signal carries the message, and how the message names
// the request. Emptiness is not checked here — each caller has its own legitimately-empty fields
// (an open-ended end date, a not-a-transfer destination, an absent description).
static QString missingOrUnreadableKeys(const QVariantMap& params, const QStringList& required)
{
    QStringList badKeys;
    for (const QString& key : required) {
        if (!params.contains(key) || !params.value(key).isValid())
            badKeys.append(key);
    }
    return badKeys.join(QStringLiteral(", "));
}

FinchClient::FinchClient(const QString& socketPath, QObject* parent)
    : QObject(parent)
{
    std::string target = "unix:" + socketPath.toStdString();
    m_channel = grpc::CreateChannel(target, grpc::InsecureChannelCredentials());
    m_stub = finch::v1::FinchService::NewStub(m_channel);

    connect(&m_pingWatcher, &QFutureWatcher<PingResult>::finished,
            this, &FinchClient::onPingFinished);
    connect(&m_accountsWatcher, &QFutureWatcher<ListAccountsResult>::finished,
            this, &FinchClient::onListAccountsFinished);
    connect(&m_createAccountWatcher, &QFutureWatcher<CreateAccountResult>::finished,
            this, &FinchClient::onCreateAccountFinished);
    connect(&m_transactionsWatcher, &QFutureWatcher<ListTransactionsResult>::finished,
            this, &FinchClient::onListTransactionsFinished);
    connect(&m_recordTransactionWatcher, &QFutureWatcher<RecordTransactionResult>::finished,
            this, &FinchClient::onRecordTransactionFinished);
    connect(&m_createRecurringRuleWatcher, &QFutureWatcher<CreateRecurringRuleResult>::finished,
            this, &FinchClient::onCreateRecurringRuleFinished);
    connect(&m_recurringRulesWatcher, &QFutureWatcher<ListRecurringRulesResult>::finished,
            this, &FinchClient::onListRecurringRulesFinished);
    connect(&m_timeSeriesWatcher, &QFutureWatcher<TimeSeriesResult>::finished,
            this, &FinchClient::onTimeSeriesFinished);
}

FinchClient::~FinchClient()
{
    m_pingWatcher.waitForFinished();
    m_accountsWatcher.waitForFinished();
    m_createAccountWatcher.waitForFinished();
    m_transactionsWatcher.waitForFinished();
    m_recordTransactionWatcher.waitForFinished();
    m_createRecurringRuleWatcher.waitForFinished();
    m_recurringRulesWatcher.waitForFinished();
    m_timeSeriesWatcher.waitForFinished();
}

void FinchClient::ping()
{
    if (m_pingInProgress)
        return;

    m_pingInProgress = true;
    emit pingInProgressChanged();

    m_connectionState = Connecting;
    emit connectionStateChanged();
    m_connectingTimer.start();

    auto stub = m_stub.get();
    auto future = QtConcurrent::run([stub]() -> PingResult {
        grpc::ClientContext context;
        context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(3));

        finch::v1::PingRequest request;
        finch::v1::PingResponse response;

        grpc::Status status = stub->Ping(&context, request, &response);
        if (status.ok()) {
            return {true, QString::fromStdString(response.version())};
        }
        return {false, {}};
    });

    m_pingWatcher.setFuture(future);
}

void FinchClient::listAccounts()
{
    if (m_accountsLoading)
        return;

    m_accountsLoading = true;
    emit accountsLoadingChanged();

    auto stub = m_stub.get();
    auto future = QtConcurrent::run([stub]() -> ListAccountsResult {
        grpc::ClientContext context;
        context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(5));

        finch::v1::ListAccountsRequest request;
        finch::v1::ListAccountsResponse response;

        grpc::Status status = stub->ListAccounts(&context, request, &response);
        if (!status.ok())
            return {false, {}};

        QVariantList accounts;
        for (const auto& account : response.accounts()) {
            QVariantMap entry;
            entry["id"] = QString::fromStdString(account.id());
            entry["name"] = QString::fromStdString(account.name());
            accounts.append(entry);
        }
        return {true, accounts};
    });

    m_accountsWatcher.setFuture(future);
}

void FinchClient::createAccount(const QString& name, int accountType)
{
    // Authoritative re-entrancy guard: the QML submit-disable is cosmetic, so without
    // this a rapid double-click could fire two CreateAccount RPCs (the daemon does not
    // prevent duplicate names) and create two accounts.
    if (m_createAccountInProgress)
        return;

    m_createAccountInProgress = true;
    emit createAccountInProgressChanged();

    auto stub = m_stub.get();
    std::string accountName = name.toStdString();
    auto future = QtConcurrent::run([stub, accountName, accountType]() -> CreateAccountResult {
        grpc::ClientContext context;
        context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(5));

        finch::v1::CreateAccountRequest request;
        request.set_name(accountName);
        request.set_type(static_cast<finch::v1::AccountType>(accountType));

        finch::v1::CreateAccountResponse response;
        grpc::Status status = stub->CreateAccount(&context, request, &response);

        CreateAccountResult result;
        if (status.ok()) {
            result.ok = true;
            result.id = QString::fromStdString(response.account().id());
            result.name = QString::fromStdString(response.account().name());
            return result;
        }

        // Surface the daemon's validation message verbatim (it is user-facing and
        // specific); for everything else show a generic message rather than a raw
        // low-level gRPC string. status only lives on this worker thread, so the
        // message is copied into the result and emitted from the GUI-thread slot.
        switch (status.error_code()) {
        case grpc::StatusCode::INVALID_ARGUMENT:
            result.errorMessage = QString::fromStdString(status.error_message());
            break;
        case grpc::StatusCode::UNAVAILABLE:
            result.errorMessage = QStringLiteral("Could not reach the daemon.");
            break;
        case grpc::StatusCode::DEADLINE_EXCEEDED:
            result.errorMessage = QStringLiteral("The request timed out. Please try again.");
            break;
        default:
            result.errorMessage = QStringLiteral("Could not create the account. Please try again.");
            break;
        }
        return result;
    });

    m_createAccountWatcher.setFuture(future);
}

void FinchClient::listTransactions(const QString& accountId)
{
    m_requestedTransactionsAccountId = accountId;
    // A fetch is already in flight; do not start a concurrent one (a second concurrent
    // QtConcurrent task would no longer be waited on by the destructor and could outlive the
    // stub). onListTransactionsFinished reconciles to the requested account once it lands.
    if (m_transactionsLoading)
        return;

    startTransactionsFetch();
}

void FinchClient::startTransactionsFetch()
{
    m_transactionsLoading = true;
    m_inFlightTransactionsAccountId = m_requestedTransactionsAccountId;
    emit transactionsLoadingChanged();

    // Clear immediately so the in-flight window never shows the previously loaded account's
    // transactions under the newly selected account (the panel binds its list to this).
    // Mirrors fetchTimeSeries clearing its data at the start of a fetch.
    m_transactions.clear();
    emit transactionsChanged();

    auto stub = m_stub.get();
    std::string id = m_inFlightTransactionsAccountId.toStdString();
    auto future = QtConcurrent::run([stub, id]() -> ListTransactionsResult {
        grpc::ClientContext context;
        context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(5));

        finch::v1::ListTransactionsRequest request;
        request.set_account_id(id);
        finch::v1::ListTransactionsResponse response;

        grpc::Status status = stub->ListTransactions(&context, request, &response);
        if (!status.ok())
            return {false, {}};

        QVariantList transactions;
        for (const auto& txn : response.transactions()) {
            QVariantMap entry;
            entry["id"] = QString::fromStdString(txn.id());
            entry["accountId"] = QString::fromStdString(txn.account_id());
            entry["date"] = QString::fromStdString(txn.date());
            // Signed cents; the panel formats and applies the sign.
            entry["amount"] = static_cast<qlonglong>(txn.amount());
            entry["name"] = QString::fromStdString(txn.name());
            entry["description"] = QString::fromStdString(txn.description());
            // Raw TransactionStatus int; the panel maps it to a human label.
            entry["status"] = static_cast<int>(txn.status());
            entry["recurringRuleId"] = QString::fromStdString(txn.recurring_rule_id());
            transactions.append(entry);
        }
        return {true, transactions};
    });

    m_transactionsWatcher.setFuture(future);
}

void FinchClient::recordTransaction(const QVariantMap& params)
{
    // Authoritative re-entrancy guard, mirroring createAccount: the QML submit-disable is
    // cosmetic, so without this a rapid double-submit could fire two RecordTransaction RPCs
    // (the daemon does not dedupe) and record two transactions.
    if (m_recordTransactionInProgress)
        return;

    // An empty description is ordinary, and the daemon rejects an empty account id, date, or name
    // loudly and specifically — so presence and readability are what this checks.
    static const QStringList requiredKeys = {
        QStringLiteral("accountId"), QStringLiteral("date"),
        QStringLiteral("amount"), QStringLiteral("name"),
        QStringLiteral("description"), QStringLiteral("status")
    };
    const QString badKeys = missingOrUnreadableKeys(params, requiredKeys);
    if (!badKeys.isEmpty()) {
        emit transactionRecordFailed(
            QStringLiteral("Internal error: transaction request missing or unreadable: %1.")
                .arg(badKeys));
        return;
    }

    // The amount is the one field whose conversion can fail on an input that looks in range.
    // Defense in depth rather than a reachable path: the magnitude field now caps both digits and
    // scale, so this guards a non-UI caller or a future host. Neither the daemon's
    // RecordTransaction nor core validates amount, so an unreadable one converting to 0 would
    // persist as a zero-value transaction.
    bool amountOk = false;
    const qlonglong amount = params.value(QStringLiteral("amount")).toLongLong(&amountOk);
    if (!amountOk) {
        emit transactionRecordFailed(
            QStringLiteral("That amount is too large. Please enter a smaller amount."));
        return;
    }
    if (amount == 0) {
        // Not reachable from the UI — the form requires a magnitude above zero — so a zero here
        // means the caller built the request wrong rather than the user mistyping.
        emit transactionRecordFailed(
            QStringLiteral("Internal error: transaction request carried a zero amount."));
        return;
    }

    // Same conversion-integrity check the amount gets; the status's *range* is left to the daemon,
    // which rejects an unspecified or out-of-range status with a specific message.
    bool statusOk = false;
    const int status = params.value(QStringLiteral("status")).toInt(&statusOk);
    if (!statusOk) {
        emit transactionRecordFailed(
            QStringLiteral("Internal error: transaction request carried an unreadable status."));
        return;
    }

    m_recordTransactionInProgress = true;
    emit recordTransactionInProgressChanged();

    auto stub = m_stub.get();
    std::string accountIdStr = params.value(QStringLiteral("accountId")).toString().toStdString();
    std::string dateStr = params.value(QStringLiteral("date")).toString().toStdString();
    std::string nameStr = params.value(QStringLiteral("name")).toString().toStdString();
    std::string descriptionStr =
        params.value(QStringLiteral("description")).toString().toStdString();
    auto future = QtConcurrent::run([stub, accountIdStr, dateStr, amount, nameStr,
                                     descriptionStr, status]() -> RecordTransactionResult {
        grpc::ClientContext context;
        context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(5));

        finch::v1::RecordTransactionRequest request;
        request.set_account_id(accountIdStr);
        request.set_date(dateStr);
        request.set_amount(amount);
        request.set_name(nameStr);
        request.set_description(descriptionStr);
        request.set_status(static_cast<finch::v1::TransactionStatus>(status));

        finch::v1::RecordTransactionResponse response;
        grpc::Status grpcStatus = stub->RecordTransaction(&context, request, &response);

        RecordTransactionResult result;
        if (grpcStatus.ok()) {
            result.ok = true;
            result.id = QString::fromStdString(response.transaction().id());
            return result;
        }

        // Surface the daemon's validation message verbatim (it is user-facing and specific);
        // everything else gets a generic message rather than a raw low-level gRPC string.
        // status only lives on this worker thread, so the message is copied into the result
        // and emitted from the GUI-thread slot.
        switch (grpcStatus.error_code()) {
        case grpc::StatusCode::INVALID_ARGUMENT:
            result.errorMessage = QString::fromStdString(grpcStatus.error_message());
            break;
        case grpc::StatusCode::UNAVAILABLE:
            result.errorMessage = QStringLiteral("Could not reach the daemon.");
            break;
        case grpc::StatusCode::DEADLINE_EXCEEDED:
            result.errorMessage = QStringLiteral("The request timed out. Please try again.");
            break;
        default:
            result.errorMessage = QStringLiteral("Could not record the transaction. Please try again.");
            break;
        }
        return result;
    });

    m_recordTransactionWatcher.setFuture(future);
}

void FinchClient::createRecurringRule(const QVariantMap& params)
{
    // Authoritative re-entrancy guard, mirroring createAccount/recordTransaction: the QML
    // submit-disable is cosmetic, so without this a rapid double-submit could create two rules.
    if (m_createRecurringRuleInProgress)
        return;

    // Presence and readability are not the whole story: a *valid* variant of the wrong type still
    // converts with a silent fallback. Where that fallback is harmless the daemon rejects it loudly
    // (an empty account id, name, or start date; an unspecified frequency), and the checks below
    // cover the two cases it would not catch. An empty end date means open-ended and an empty
    // destination means not-a-transfer, so emptiness is deliberately not a failure here.
    static const QStringList requiredKeys = {
        QStringLiteral("accountId"), QStringLiteral("name"),
        QStringLiteral("amount"), QStringLiteral("frequency"),
        QStringLiteral("startDate"), QStringLiteral("endDate"),
        QStringLiteral("dayOfMonth"), QStringLiteral("semiMonthlyDays"),
        QStringLiteral("isTransfer"), QStringLiteral("transferTargetAccountId")
    };
    const QString badKeys = missingOrUnreadableKeys(params, requiredKeys);
    if (!badKeys.isEmpty()) {
        emit recurringRuleCreateFailed(
            QStringLiteral("Internal error: rule request missing or unreadable: %1.")
                .arg(badKeys));
        return;
    }

    // The amount is the one field whose conversion can fail on an input that still looks numeric —
    // an Infinity, or a magnitude outside int64 — and a failed conversion yields 0.
    // Neither the daemon nor core validates a non-transfer amount, so a 0 would persist as a
    // rule that silently projects nothing.
    bool amountOk = false;
    const qlonglong amount = params.value(QStringLiteral("amount")).toLongLong(&amountOk);
    if (!amountOk) {
        // Defense in depth, not a reachable path: the magnitude field caps both digits and scale,
        // so this guards a non-UI caller or a future host rather than ordinary user input.
        emit recurringRuleCreateFailed(
            QStringLiteral("That amount is too large. Please enter a smaller amount."));
        return;
    }
    if (amount == 0) {
        // Not reachable from the UI — the form requires a magnitude above zero — so a zero here
        // means the caller built the request wrong rather than the user mistyping.
        emit recurringRuleCreateFailed(
            QStringLiteral("Internal error: rule request carried a zero amount."));
        return;
    }

    // The other two numeric fields get the same conversion-integrity check the amount does.
    // Their *range* is deliberately left to the daemon, which already rejects an unspecified or
    // out-of-range frequency and an out-of-range day-of-month with a specific message — this
    // guard is about a value that could not be read at all, not about domain validity.
    bool frequencyOk = false;
    const int frequency = params.value(QStringLiteral("frequency")).toInt(&frequencyOk);
    bool dayOfMonthOk = false;
    const int dayOfMonth = params.value(QStringLiteral("dayOfMonth")).toInt(&dayOfMonthOk);
    if (!frequencyOk || !dayOfMonthOk) {
        emit recurringRuleCreateFailed(
            QStringLiteral("Internal error: rule request carried an unreadable "
                           "frequency or day-of-month."));
        return;
    }

    // The transfer flag and the destination are one fact expressed twice, and the caller derives
    // the flag *from* the destination — so they must agree. Checking the invariant rather than
    // the flag's type catches the one silently-corrupting case here no matter its cause: a flag
    // that reads false while a destination is set persists an outbound transfer as ordinary
    // income on its source account, with a phantom destination stored alongside it. A type
    // assertion would catch only the wrong-type spelling of that mistake; this catches a wrong
    // property, a stale expression, and an inverted condition too.
    const bool isTransfer = params.value(QStringLiteral("isTransfer")).toBool();
    const QString transferTarget =
        params.value(QStringLiteral("transferTargetAccountId")).toString();
    if (isTransfer != !transferTarget.isEmpty()) {
        emit recurringRuleCreateFailed(
            QStringLiteral("Internal error: rule request's transfer flag and destination "
                           "disagree."));
        return;
    }

    m_createRecurringRuleInProgress = true;
    emit createRecurringRuleInProgressChanged();

    auto stub = m_stub.get();
    std::string accountIdStr = params.value(QStringLiteral("accountId")).toString().toStdString();
    std::string nameStr = params.value(QStringLiteral("name")).toString().toStdString();
    std::string startDateStr = params.value(QStringLiteral("startDate")).toString().toStdString();
    std::string endDateStr = params.value(QStringLiteral("endDate")).toString().toStdString();
    std::string transferTargetStr = transferTarget.toStdString();
    QList<int> semiDays;
    for (const auto& d : params.value(QStringLiteral("semiMonthlyDays")).toList())
        semiDays.append(d.toInt());
    auto future = QtConcurrent::run([stub, accountIdStr, nameStr, amount, frequency,
                                     startDateStr, endDateStr, dayOfMonth, isTransfer,
                                     transferTargetStr,
                                     semiDays]() -> CreateRecurringRuleResult {
        grpc::ClientContext context;
        context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(5));

        finch::v1::CreateRecurringRuleRequest request;
        request.set_account_id(accountIdStr);
        request.set_name(nameStr);
        request.set_amount(amount);
        request.set_frequency(static_cast<finch::v1::Frequency>(frequency));
        request.set_start_date(startDateStr);
        request.set_end_date(endDateStr);
        request.set_day_of_month(dayOfMonth);
        for (int d : semiDays)
            request.add_semi_monthly_days(d);
        // A transfer's amount is a positive magnitude and its direction comes from the
        // source/target pair, so the form sends the unsigned value and the daemon rejects a
        // non-positive one. Both fields go together: the daemon requires a target whenever
        // is_transfer is set.
        request.set_is_transfer(isTransfer);
        request.set_transfer_target_account_id(transferTargetStr);

        finch::v1::CreateRecurringRuleResponse response;
        grpc::Status grpcStatus = stub->CreateRecurringRule(&context, request, &response);

        CreateRecurringRuleResult result;
        if (grpcStatus.ok()) {
            result.ok = true;
            result.id = QString::fromStdString(response.rule().id());
            return result;
        }

        // Surface the daemon's validation message verbatim (user-facing and specific — the
        // daemon now returns InvalidArgument for out-of-range day fields); everything else gets
        // a generic message. status lives only on this worker thread, so copy it into result.
        switch (grpcStatus.error_code()) {
        case grpc::StatusCode::INVALID_ARGUMENT:
            result.errorMessage = QString::fromStdString(grpcStatus.error_message());
            break;
        case grpc::StatusCode::UNAVAILABLE:
            result.errorMessage = QStringLiteral("Could not reach the daemon.");
            break;
        case grpc::StatusCode::DEADLINE_EXCEEDED:
            result.errorMessage = QStringLiteral("The request timed out. Please try again.");
            break;
        default:
            result.errorMessage = QStringLiteral("Could not create the rule. Please try again.");
            break;
        }
        return result;
    });

    m_createRecurringRuleWatcher.setFuture(future);
}

void FinchClient::listRecurringRules(const QString& accountId)
{
    m_requestedRecurringRulesAccountId = accountId;
    // A fetch is already in flight; do not start a concurrent one (a second QtConcurrent task
    // would no longer be waited on by the destructor). onListRecurringRulesFinished reconciles
    // to the requested account once it lands.
    if (m_recurringRulesLoading)
        return;

    startRecurringRulesFetch();
}

void FinchClient::startRecurringRulesFetch()
{
    m_recurringRulesLoading = true;
    m_inFlightRecurringRulesAccountId = m_requestedRecurringRulesAccountId;
    emit recurringRulesLoadingChanged();

    // Clear immediately so the in-flight window never shows the previously loaded account's
    // rules under the newly selected account (the panel binds its list to this).
    m_recurringRules.clear();
    emit recurringRulesChanged();

    auto stub = m_stub.get();
    std::string id = m_inFlightRecurringRulesAccountId.toStdString();
    auto future = QtConcurrent::run([stub, id]() -> ListRecurringRulesResult {
        grpc::ClientContext context;
        context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(5));

        finch::v1::ListRecurringRulesRequest request;
        request.set_account_id(id);
        finch::v1::ListRecurringRulesResponse response;

        grpc::Status status = stub->ListRecurringRules(&context, request, &response);
        if (!status.ok())
            return {false, {}};

        QVariantList rules;
        for (const auto& rule : response.rules()) {
            QVariantMap entry;
            entry["id"] = QString::fromStdString(rule.id());
            entry["accountId"] = QString::fromStdString(rule.account_id());
            entry["name"] = QString::fromStdString(rule.name());
            // Signed cents for an ordinary rule, but a POSITIVE magnitude for a transfer, whose
            // direction comes from the source/target pair rather than the stored sign — see
            // ruleAmount in txformat.js, which mirrors core's effectOn.
            entry["amount"] = static_cast<qlonglong>(rule.amount());
            // Raw Frequency int; the panel maps it to a human label.
            entry["frequency"] = static_cast<int>(rule.frequency());
            entry["startDate"] = QString::fromStdString(rule.start_date());
            entry["endDate"] = QString::fromStdString(rule.end_date());
            entry["dayOfMonth"] = static_cast<int>(rule.day_of_month());
            QVariantList semiDays;
            for (int d : rule.semi_monthly_days())
                semiDays.append(d);
            entry["semiMonthlyDays"] = semiDays;
            entry["isTransfer"] = rule.is_transfer();
            entry["transferTargetAccountId"] = QString::fromStdString(rule.transfer_target_account_id());
            entry["paused"] = rule.paused();
            rules.append(entry);
        }
        return {true, rules};
    });

    m_recurringRulesWatcher.setFuture(future);
}

void FinchClient::fetchTimeSeries(const QString& fromDate, const QString& toDate,
                                  int interval, const QStringList& accountIds)
{
    if (m_timeSeriesLoading)
        return;

    m_timeSeriesLoading = true;
    emit timeSeriesLoadingChanged();

    m_timeSeriesData = {};
    emit timeSeriesDataChanged();

    auto stub = m_stub.get();
    std::string from = fromDate.toStdString();
    std::string to = toDate.toStdString();
    auto protoInterval = static_cast<finch::v1::TimeSeriesInterval>(interval);
    std::vector<std::string> ids;
    for (const auto& id : accountIds)
        ids.push_back(id.toStdString());

    auto future = QtConcurrent::run([stub, from, to, protoInterval, ids]() -> TimeSeriesResult {
        grpc::ClientContext context;
        context.set_deadline(std::chrono::system_clock::now() + std::chrono::seconds(10));

        finch::v1::GetBalanceTimeSeriesRequest request;
        request.set_from_date(from);
        request.set_to_date(to);
        request.set_interval(protoInterval);
        for (const auto& id : ids)
            request.add_account_ids(id);

        finch::v1::GetBalanceTimeSeriesResponse response;
        grpc::Status status = stub->GetBalanceTimeSeries(&context, request, &response);
        if (!status.ok())
            return {};

        TimeSeriesResult result;
        result.ok = true;
        for (const auto& point : response.points()) {
            QDate date = QDate::fromString(QString::fromStdString(point.date()), "yyyy-MM-dd");
            if (!date.isValid())
                continue;
            qint64 msec = QDateTime(date, QTime(0, 0), Qt::UTC).toMSecsSinceEpoch();

            for (const auto& ab : point.balances()) {
                QString accountId = QString::fromStdString(ab.account_id());
                double dollars = static_cast<double>(ab.balance()) / 100.0;
                result.seriesByAccount[accountId].append({msec, dollars});
            }
        }
        return result;
    });

    m_timeSeriesWatcher.setFuture(future);
}

void FinchClient::populateSeries(QObject* seriesObj, const QString& accountId)
{
    auto series = qobject_cast<QXYSeries*>(seriesObj);
    if (!series)
        return;

    const auto it = m_timeSeriesData.seriesByAccount.find(accountId);
    if (it == m_timeSeriesData.seriesByAccount.end()) {
        series->clear();
        return;
    }

    QList<QPointF> points;
    points.reserve(it->size());
    for (const auto& pt : *it)
        points.append(QPointF(pt.msecsSinceEpoch, pt.balance));

    series->replace(points);
}

bool FinchClient::timeSeriesEmpty() const
{
    return !m_timeSeriesData.ok || m_timeSeriesData.seriesByAccount.isEmpty();
}

QStringList FinchClient::timeSeriesAccountIds() const
{
    return m_timeSeriesData.seriesByAccount.keys();
}

double FinchClient::timeSeriesMinDate() const
{
    double min = std::numeric_limits<double>::max();
    for (const auto& points : m_timeSeriesData.seriesByAccount) {
        if (!points.isEmpty())
            min = qMin(min, static_cast<double>(points.first().msecsSinceEpoch));
    }
    return min == std::numeric_limits<double>::max() ? 0.0 : min;
}

double FinchClient::timeSeriesMaxDate() const
{
    double max = std::numeric_limits<double>::lowest();
    for (const auto& points : m_timeSeriesData.seriesByAccount) {
        if (!points.isEmpty())
            max = qMax(max, static_cast<double>(points.last().msecsSinceEpoch));
    }
    return max == std::numeric_limits<double>::lowest() ? 0.0 : max;
}

double FinchClient::timeSeriesMinBalance() const
{
    double min = std::numeric_limits<double>::max();
    double max = std::numeric_limits<double>::lowest();
    for (const auto& points : m_timeSeriesData.seriesByAccount) {
        for (const auto& pt : points) {
            min = qMin(min, pt.balance);
            max = qMax(max, pt.balance);
        }
    }
    if (min == std::numeric_limits<double>::max())
        return 0.0;
    double range = max - min;
    return min - range * 0.05;
}

double FinchClient::timeSeriesMaxBalance() const
{
    double min = std::numeric_limits<double>::max();
    double max = std::numeric_limits<double>::lowest();
    for (const auto& points : m_timeSeriesData.seriesByAccount) {
        for (const auto& pt : points) {
            min = qMin(min, pt.balance);
            max = qMax(max, pt.balance);
        }
    }
    if (max == std::numeric_limits<double>::lowest())
        return 0.0;
    double range = max - min;
    return max + range * 0.05;
}

void FinchClient::onListAccountsFinished()
{
    auto result = m_accountsWatcher.result();

    m_accountsLoading = false;
    emit accountsLoadingChanged();

    // A create completed while this fetch was in flight, so this result predates the new
    // account and would clobber the entry appended in onCreateAccountFinished. Discard the
    // stale list and re-fetch the authoritative one (which now includes the new account).
    if (m_accountsRefreshPending) {
        m_accountsRefreshPending = false;
        listAccounts();
        return;
    }

    if (result.ok) {
        m_accounts = result.accounts;
    } else {
        m_accounts.clear();
    }
    emit accountsChanged();
}

void FinchClient::onCreateAccountFinished()
{
    auto result = m_createAccountWatcher.result();

    m_createAccountInProgress = false;
    emit createAccountInProgressChanged();

    if (!result.ok) {
        emit accountCreateFailed(result.errorMessage);
        return;
    }

    // Append the created account to the model directly rather than refreshing via
    // listAccounts(): that method's own in-progress guard would silently drop a refresh
    // issued while a list is already in flight, leaving a successful create invisible.
    QVariantMap entry;
    entry["id"] = result.id;
    entry["name"] = result.name;
    m_accounts.append(entry);
    emit accountsChanged();

    // If a listAccounts() is in flight, its result predates this create and would
    // overwrite the appended entry; flag it so onListAccountsFinished reconciles.
    if (m_accountsLoading)
        m_accountsRefreshPending = true;

    emit accountCreated(result.id);
}

void FinchClient::onListTransactionsFinished()
{
    auto result = m_transactionsWatcher.result();

    m_transactionsLoading = false;
    emit transactionsLoadingChanged();

    // A newer account was selected while this fetch was in flight (a rapid account switch);
    // its result is for the wrong account. Discard it and fetch the account now selected.
    // This input-ordering race is independent of the read-only nature of the feature — it is
    // about which account the user wants, not about a mutation racing the read.
    if (m_inFlightTransactionsAccountId != m_requestedTransactionsAccountId) {
        startTransactionsFetch();
        return;
    }

    // A failed fetch clears the list — the panel's empty/state message covers it; a dedicated
    // error surface rides along with the later transaction-recording work, where write
    // failures make it load-bearing.
    if (result.ok) {
        m_transactions = result.transactions;
    } else {
        m_transactions.clear();
    }
    emit transactionsChanged();
}

void FinchClient::onRecordTransactionFinished()
{
    auto result = m_recordTransactionWatcher.result();

    m_recordTransactionInProgress = false;
    emit recordTransactionInProgressChanged();

    if (!result.ok) {
        emit transactionRecordFailed(result.errorMessage);
        return;
    }

    emit transactionRecorded(result.id);
}

void FinchClient::onCreateRecurringRuleFinished()
{
    auto result = m_createRecurringRuleWatcher.result();

    m_createRecurringRuleInProgress = false;
    emit createRecurringRuleInProgressChanged();

    if (!result.ok) {
        emit recurringRuleCreateFailed(result.errorMessage);
        return;
    }

    // Emit only — the panel re-fetches the account's rules on this signal (mirroring the
    // transaction write path). Do not append to a local model like createAccount does: an
    // append would be clobbered by the re-fetch's list-clear.
    emit recurringRuleCreated(result.id);
}

void FinchClient::onListRecurringRulesFinished()
{
    auto result = m_recurringRulesWatcher.result();

    m_recurringRulesLoading = false;
    emit recurringRulesLoadingChanged();

    // A newer account was selected while this fetch was in flight (a rapid account switch); its
    // result is for the wrong account. Discard it and fetch the account now selected.
    if (m_inFlightRecurringRulesAccountId != m_requestedRecurringRulesAccountId) {
        startRecurringRulesFetch();
        return;
    }

    if (result.ok) {
        m_recurringRules = result.rules;
    } else {
        m_recurringRules.clear();
    }
    emit recurringRulesChanged();
}

void FinchClient::onPingFinished()
{
    m_pendingResult = m_pingWatcher.result();

    qint64 elapsed = m_connectingTimer.elapsed();
    if (elapsed < kMinConnectingMs) {
        QTimer::singleShot(kMinConnectingMs - elapsed, this, &FinchClient::applyPingResult);
    } else {
        applyPingResult();
    }
}

void FinchClient::applyPingResult()
{
    m_pingInProgress = false;
    emit pingInProgressChanged();

    if (m_pendingResult.ok) {
        m_version = m_pendingResult.version;
        m_connectionState = Connected;
    } else {
        m_version.clear();
        m_connectionState = Disconnected;
    }

    emit daemonVersionChanged();
    emit connectionStateChanged();
}

void FinchClient::onTimeSeriesFinished()
{
    m_timeSeriesData = m_timeSeriesWatcher.result();
    m_timeSeriesLoading = false;
    emit timeSeriesLoadingChanged();
    emit timeSeriesDataChanged();
}
