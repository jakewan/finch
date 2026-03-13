#ifndef FINCHCLIENT_H
#define FINCHCLIENT_H

#include <QObject>
#include <QString>
#include <memory>

#include <grpcpp/grpcpp.h>
#include "finch/v1/finch.grpc.pb.h"

class FinchClient : public QObject {
    Q_OBJECT
    Q_PROPERTY(QString daemonVersion READ daemonVersion NOTIFY daemonVersionChanged)

public:
    explicit FinchClient(const QString& socketPath, QObject* parent = nullptr);

    QString daemonVersion() const { return m_version; }
    Q_INVOKABLE bool ping();

signals:
    void daemonVersionChanged();

private:
    std::shared_ptr<grpc::Channel> m_channel;
    std::unique_ptr<finch::v1::FinchService::Stub> m_stub;
    QString m_version;
};

#endif // FINCHCLIENT_H
