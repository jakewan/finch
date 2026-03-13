#include "finchclient.h"
#include <grpcpp/grpcpp.h>

FinchClient::FinchClient(const QString& socketPath, QObject* parent)
    : QObject(parent)
{
    std::string target = "unix:" + socketPath.toStdString();
    m_channel = grpc::CreateChannel(target, grpc::InsecureChannelCredentials());
    m_stub = finch::v1::FinchService::NewStub(m_channel);
}

bool FinchClient::ping()
{
    grpc::ClientContext context;
    finch::v1::PingRequest request;
    finch::v1::PingResponse response;

    grpc::Status status = m_stub->Ping(&context, request, &response);
    if (status.ok()) {
        m_version = QString::fromStdString(response.version());
        emit daemonVersionChanged();
        return true;
    }
    return false;
}
