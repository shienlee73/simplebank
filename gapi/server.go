package gapi

import (
	"fmt"

	db "github.com/shienlee73/simplebank/db/sqlc"
	pb "github.com/shienlee73/simplebank/pb"
	"github.com/shienlee73/simplebank/token"
	"github.com/shienlee73/simplebank/util"
	"github.com/shienlee73/simplebank/worker"
)

type Server struct {
	pb.UnimplementedSimpleBankServer
	store      db.Store
	tokenMaker token.Maker
	config     util.Config
	taskDistributor worker.TaskDistributor
}

func NewServer(config util.Config, store db.Store, taskDistributor worker.TaskDistributor) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}

	server := &Server{store: store, tokenMaker: tokenMaker, config: config, taskDistributor: taskDistributor}

	return server, nil
}
