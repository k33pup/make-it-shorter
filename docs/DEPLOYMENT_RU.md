# Руководство по развертыванию (Deployment Guide)

В этом руководстве описано, как развернуть проект `make-it-shorter` на Linux-сервере (например, Ubuntu) с использованием Nginx в качестве шлюза и автоматическим получением SSL-сертификатов от Let's Encrypt.

## 1. Предварительные требования

1.  **Сервер**: VPS/VDS с Linux (рекомендуется Ubuntu 20.04 или 22.04).
2.  **Домен**: Купленное доменное имя (например, `mysite.com`).
3.  **DNS**: А-запись вашего домена должна указывать на IP-адрес вашего сервера.

## 2. Подготовка сервера

Зайдите на сервер по SSH и установите Docker и Docker Compose.

```bash
# Обновите списки пакетов
sudo apt update
sudo apt upgrade -y

# Установите Docker и Docker Compose
sudo apt install docker.io docker-compose -y

# Запустите Docker и добавьте его в автозагрузку
sudo systemctl start docker
sudo systemctl enable docker

# (Опционально) Добавьте текущего пользователя в группу docker, чтобы не писать sudo каждый раз
sudo usermod -aG docker $USER
# После этого нужно перелогиниться
```

## 3. Настройка Firewall (UFW)

Откройте необходимые порты:

```bash
sudo ufw allow 22/tcp  # SSH
sudo ufw allow 80/tcp  # HTTP
sudo ufw allow 443/tcp # HTTPS
sudo ufw enable
```

## 4. Загрузка проекта

Склонируйте репозиторий на сервер или скопируйте файлы.

```bash
git clone https://github.com/your-username/make-it-shorter.git
cd make-it-shorter
```

## 5. Конфигурация домена

Вам нужно отредактировать два файла, заменив `example.com` на ваш реальный домен.

### Шаг 5.1: Настройка скрипта инициализации SSL

Откройте файл `init-letsencrypt.sh`:

```bash
nano init-letsencrypt.sh
```

Найдите строку:
```bash
domains=(example.com)
```
Замените `example.com` на ваш домен.
Также рекомендуется указать email для уведомлений (строка `email=""`).

### Шаг 5.2: Настройка Nginx

Скопируйте шаблон конфигурации:

```bash
cp nginx/conf/app.conf.template nginx/conf/app.conf
```

Откройте файл `nginx/conf/app.conf`:

```bash
nano nginx/conf/app.conf
```

Замените **все** упоминания `example.com` на ваш домен (их там несколько: в `server_name` и путях к сертификатам).

## 6. Запуск

Запустите скрипт инициализации. Он создаст "заглушечные" сертификаты, запустит Nginx, а затем запросит реальные сертификаты у Let's Encrypt.

```bash
sudo ./init-letsencrypt.sh
```

После успешного выполнения скрипта ваш проект будет доступен по HTTPS.

## 7. Обслуживание

### Просмотр логов
```bash
docker-compose logs -f
```

### Перезапуск
Если вы изменили код приложения:
```bash
docker-compose up -d --build
```

### Обновление сертификатов
Certbot настроен на автоматическое обновление сертификатов. Скрипт в контейнере проверяет их каждые 12 часов.
