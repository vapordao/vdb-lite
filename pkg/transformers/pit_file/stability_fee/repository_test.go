package stability_fee_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres/repositories"
	"github.com/vulcanize/vulcanizedb/pkg/transformers/pit_file/stability_fee"
	"github.com/vulcanize/vulcanizedb/pkg/transformers/test_data"
	"github.com/vulcanize/vulcanizedb/test_config"
)

var _ = Describe("", func() {
	Describe("Create", func() {
		It("adds a pit file stability fee event", func() {
			db := test_config.NewTestDB(core.Node{})
			test_config.CleanTestDB(db)
			headerRepository := repositories.NewHeaderRepository(db)
			headerID, err := headerRepository.CreateOrUpdateHeader(core.Header{})
			Expect(err).NotTo(HaveOccurred())
			pitFileRepository := stability_fee.NewPitFileStabilityFeeRepository(db)

			err = pitFileRepository.Create(headerID, test_data.PitFileStabilityFeeModel)

			Expect(err).NotTo(HaveOccurred())
			var dbPitFile stability_fee.PitFileStabilityFeeModel
			err = db.Get(&dbPitFile, `SELECT what, data, tx_idx, raw_log FROM maker.pit_file_stability_fee WHERE header_id = $1`, headerID)
			Expect(err).NotTo(HaveOccurred())
			Expect(dbPitFile.What).To(Equal(test_data.PitFileStabilityFeeModel.What))
			Expect(dbPitFile.Data).To(Equal(test_data.PitFileStabilityFeeModel.Data))
			Expect(dbPitFile.TransactionIndex).To(Equal(test_data.PitFileStabilityFeeModel.TransactionIndex))
			Expect(dbPitFile.Raw).To(MatchJSON(test_data.PitFileStabilityFeeModel.Raw))
		})

		It("does not duplicate pit file events", func() {
			db := test_config.NewTestDB(core.Node{})
			test_config.CleanTestDB(db)
			headerRepository := repositories.NewHeaderRepository(db)
			headerID, err := headerRepository.CreateOrUpdateHeader(core.Header{})
			Expect(err).NotTo(HaveOccurred())
			pitFileRepository := stability_fee.NewPitFileStabilityFeeRepository(db)
			err = pitFileRepository.Create(headerID, test_data.PitFileStabilityFeeModel)
			Expect(err).NotTo(HaveOccurred())

			err = pitFileRepository.Create(headerID, test_data.PitFileStabilityFeeModel)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pq: duplicate key value violates unique constraint"))
		})

		It("removes pit file if corresponding header is deleted", func() {
			db := test_config.NewTestDB(core.Node{})
			test_config.CleanTestDB(db)
			headerRepository := repositories.NewHeaderRepository(db)
			headerID, err := headerRepository.CreateOrUpdateHeader(core.Header{})
			Expect(err).NotTo(HaveOccurred())
			pitFileRepository := stability_fee.NewPitFileStabilityFeeRepository(db)
			err = pitFileRepository.Create(headerID, test_data.PitFileStabilityFeeModel)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`DELETE FROM headers WHERE id = $1`, headerID)

			Expect(err).NotTo(HaveOccurred())
			var dbPitFile stability_fee.PitFileStabilityFeeModel
			err = db.Get(&dbPitFile, `SELECT what, data, tx_idx, raw_log FROM maker.pit_file_stability_fee WHERE header_id = $1`, headerID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(sql.ErrNoRows))
		})
	})

	Describe("MissingHeaders", func() {
		It("returns headers with no associated pit file event", func() {
			db := test_config.NewTestDB(core.Node{})
			test_config.CleanTestDB(db)
			headerRepository := repositories.NewHeaderRepository(db)
			startingBlockNumber := int64(1)
			pitFileBlockNumber := int64(2)
			endingBlockNumber := int64(3)
			blockNumbers := []int64{startingBlockNumber, pitFileBlockNumber, endingBlockNumber, endingBlockNumber + 1}
			var headerIDs []int64
			for _, n := range blockNumbers {
				headerID, err := headerRepository.CreateOrUpdateHeader(core.Header{BlockNumber: n})
				headerIDs = append(headerIDs, headerID)
				Expect(err).NotTo(HaveOccurred())
			}
			pitFileRepository := stability_fee.NewPitFileStabilityFeeRepository(db)
			err := pitFileRepository.Create(headerIDs[1], test_data.PitFileStabilityFeeModel)
			Expect(err).NotTo(HaveOccurred())

			headers, err := pitFileRepository.MissingHeaders(startingBlockNumber, endingBlockNumber)

			Expect(err).NotTo(HaveOccurred())
			Expect(len(headers)).To(Equal(2))
			Expect(headers[0].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber)))
			Expect(headers[1].BlockNumber).To(Or(Equal(startingBlockNumber), Equal(endingBlockNumber)))
		})

		It("only returns headers associated with the current node", func() {
			db := test_config.NewTestDB(core.Node{})
			test_config.CleanTestDB(db)
			blockNumbers := []int64{1, 2, 3}
			headerRepository := repositories.NewHeaderRepository(db)
			dbTwo := test_config.NewTestDB(core.Node{ID: "second"})
			headerRepositoryTwo := repositories.NewHeaderRepository(dbTwo)
			var headerIDs []int64
			for _, n := range blockNumbers {
				headerID, err := headerRepository.CreateOrUpdateHeader(core.Header{BlockNumber: n})
				Expect(err).NotTo(HaveOccurred())
				headerIDs = append(headerIDs, headerID)
				_, err = headerRepositoryTwo.CreateOrUpdateHeader(core.Header{BlockNumber: n})
				Expect(err).NotTo(HaveOccurred())
			}
			pitFileRepository := stability_fee.NewPitFileStabilityFeeRepository(db)
			pitFileRepositoryTwo := stability_fee.NewPitFileStabilityFeeRepository(dbTwo)
			err := pitFileRepository.Create(headerIDs[0], test_data.PitFileStabilityFeeModel)
			Expect(err).NotTo(HaveOccurred())

			nodeOneMissingHeaders, err := pitFileRepository.MissingHeaders(blockNumbers[0], blockNumbers[len(blockNumbers)-1])
			Expect(err).NotTo(HaveOccurred())
			Expect(len(nodeOneMissingHeaders)).To(Equal(len(blockNumbers) - 1))

			nodeTwoMissingHeaders, err := pitFileRepositoryTwo.MissingHeaders(blockNumbers[0], blockNumbers[len(blockNumbers)-1])
			Expect(err).NotTo(HaveOccurred())
			Expect(len(nodeTwoMissingHeaders)).To(Equal(len(blockNumbers)))
		})
	})
})