#include "finchclient.h"

#include <QTimer>
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
}

FinchClient::~FinchClient()
{
    // Wait for in-flight ping to complete before destroying the stub.
    // Bounded by the 3-second gRPC deadline.
    m_pingWatcher.waitForFinished();
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
