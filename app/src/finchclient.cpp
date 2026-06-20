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
    connect(&m_timeSeriesWatcher, &QFutureWatcher<TimeSeriesResult>::finished,
            this, &FinchClient::onTimeSeriesFinished);
}

FinchClient::~FinchClient()
{
    m_pingWatcher.waitForFinished();
    m_accountsWatcher.waitForFinished();
    m_createAccountWatcher.waitForFinished();
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

    emit accountCreated(result.id);
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
