#include "finchclient.h"

#include <QTimer>
#include <QVariantMap>
#include <QtConcurrent>
#include <chrono>
#include <grpcpp/grpcpp.h>

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
}

FinchClient::~FinchClient()
{
    m_pingWatcher.waitForFinished();
    m_accountsWatcher.waitForFinished();
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
