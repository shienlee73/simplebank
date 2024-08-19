package main

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hibiken/asynq"
	_ "github.com/lib/pq"
	"github.com/rakyll/statik/fs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/shienlee73/simplebank/api"
	db "github.com/shienlee73/simplebank/db/sqlc"
	_ "github.com/shienlee73/simplebank/doc/statik"
	"github.com/shienlee73/simplebank/gapi"
	"github.com/shienlee73/simplebank/mail"
	pb "github.com/shienlee73/simplebank/pb"
	"github.com/shienlee73/simplebank/util"
	"github.com/shienlee73/simplebank/worker"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

var interruptSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGINT,
}

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot load config")
	}

	if config.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	ctx, stop := signal.NotifyContext(context.Background(), interruptSignals...)
	defer stop()

	conn, err := sql.Open(config.DBDriver, config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}

	// run db migration
	runDBMigration(config.MigrationURL, config.DBSource)

	store := db.NewStore(conn)

	redisOpt := asynq.RedisClientOpt{
		Addr: config.RedisAddress,
	}

	waitGroup, ctx := errgroup.WithContext(ctx)

	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)
	go runTaskProcessor(ctx, waitGroup, config, redisOpt, store)

	go runGatewayServer(ctx, waitGroup, config, store, taskDistributor)
	runGrpcServer(ctx, waitGroup, config, store, taskDistributor)

	err = waitGroup.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("error from wait group")
	}
}

func runDBMigration(migrationURL string, dbSource string) {
	m, err := migrate.New(migrationURL, dbSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create migrate instance")
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal().Err(err).Msg("failed to run migrate up")
	}
	log.Info().Msg("db migrated successfully")
}

func runTaskProcessor(ctx context.Context, waitGroup *errgroup.Group, config util.Config, redisOpt asynq.RedisClientOpt, store db.Store) {
	mailer := mail.NewGamilSender(config.EmailSenderName, config.EmailSenderAddress, config.EmailSenderPassword)
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, mailer)

	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}
	log.Info().Msg("start task processor")

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shotdown task processor")

		taskProcessor.Shutdown()
		log.Info().Msg("task processor is stopped")

		return nil
	})
}

func runGatewayServer(ctx context.Context, waitGroup *errgroup.Group, config util.Config, store db.Store, taskDistributor worker.TaskDistributor) {
	server, err := gapi.NewServer(config, store, taskDistributor)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create server")
	}

	jsonOption := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames: true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	})

	grpcMux := runtime.NewServeMux(jsonOption)

	err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, server)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot register handler")
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	statikFS, err := fs.New()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create statik fs")
	}

	swaggerHandler := http.StripPrefix("/swagger/", http.FileServer(statikFS))
	mux.Handle("/swagger/", swaggerHandler)

	httpServer := &http.Server{
		Handler: gapi.HttpLogger(mux),
		Addr:    config.HTTPServerAddress,
	}

	waitGroup.Go(func() error {
		log.Info().Msgf("start HTTP gateway server at %s", httpServer.Addr)
		err = httpServer.ListenAndServe()
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			log.Error().Err(err).Msg("HTTP gateway server failed to serve")
			return err
		}
		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown HTTP gateway server")
		
		err := httpServer.Shutdown(context.Background())
		if err != nil {
			log.Info().Msg("failed to shutdown HTTP gateway server")
			return err
		}

		log.Info().Msg("HTTP gateway server is stopped")
		return nil
	})
}

func runGrpcServer(ctx context.Context, waitGroup *errgroup.Group, config util.Config, store db.Store, taskDistributor worker.TaskDistributor) {
	server, err := gapi.NewServer(config, store, taskDistributor)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create server")
	}

	grpcLogger := grpc.UnaryInterceptor(gapi.GrpcLogger)
	grpcServer := grpc.NewServer(grpcLogger)
	pb.RegisterSimpleBankServer(grpcServer, server)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", config.GRPCServerAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create listener")
	}

	waitGroup.Go(func() error {
		log.Info().Msgf("start gRPC server at %s", listener.Addr().String())
		err = grpcServer.Serve(listener)
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			log.Error().Err(err).Msg("cannot start gRPC server")
		}
		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown gRPC server")

		grpcServer.GracefulStop()
		log.Info().Msg("gRPC server is stopped")

		return nil
	})
}

func runGinServer(config util.Config, store db.Store) {
	server, err := api.NewServer(config, store)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create server")
	}

	if err = server.Start(config.HTTPServerAddress); err != nil {
		log.Fatal().Err(err).Msg("cannot start HTTP server")
	}
}
