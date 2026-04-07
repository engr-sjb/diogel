/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package capsule

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/engr-sjb/diogel/internal/peererrors"
	"github.com/engr-sjb/diogel/internal/transport"
)

// incoming

var (
	ErrInvalidCapsuleMasterKeyRecoveryThreshold = errors.New("")
)

type CreateCapsuleDTO struct {
	RemotePeerGuardians               []transport.RemotePeer
	RemotePeerStorageProviders        []transport.RemotePeer
	SilencePeriod                     time.Duration
	Letter                            ports.File
	FilePaths                         []string
	CapsuleMasterKeyRecoveryThreshold int
}

func (cc *CreateCapsuleDTO) validate(d Defaults) error {
	// todo: we need to validate the whole struct and return the all the errors together
	if len(cc.RemotePeerGuardians) < int(d.MinNumOfGuardians) {
		return peererrors.New(
			peererrors.CodeLocalPeerError,
			fmt.Sprintf(
				"guardians must be at least %d",
				d.MinNumOfGuardians,
			),
			ErrInvalidGuardiansCount,
			featureCapsule,
		)
	}
	if len(cc.RemotePeerGuardians) > int(d.MaxNumOfGuardians) {
		return peererrors.New(
			peererrors.CodeLocalPeerError,
			fmt.Sprintf(
				"guardians must be at most %d",
				d.MaxNumOfGuardians,
			),
			ErrInvalidGuardiansCount,
			featureCapsule,
		)
	}

	if cc.SilencePeriod == 0 {
		cc.SilencePeriod = defaultSilencePeriod
	}

	hasLetter := cc.Letter != nil
	log.Println(hasLetter, "<<< hasLetter")
	hasFilePaths := len(cc.FilePaths) > 0 && cc.FilePaths[0] != ""

	if !hasLetter && !hasFilePaths {
		return peererrors.New(
			peererrors.CodeLocalPeerError,
			"at least, a letter or a file path(s) must be provided",
			ErrInvalidCreateCapsuleData,
			featureCapsule,
		)
	}

	// Todo: Might have to create a error type to have multiple errors as i would love for we to see all file that don't exist in one go and return them as error.
	if hasFilePaths {
		for i := range cc.FilePaths {
			if _, err := os.Stat(cc.FilePaths[i]); os.IsNotExist(err) {
				return peererrors.New(
					peererrors.CodeLocalPeerError,
					fmt.Sprintf("file does not exist: %s", cc.FilePaths[i]),
					err,
					featureCapsule,
				)
			}
		}

		// for i := range cc.FilePaths {
		// 		if _, err := os.Stat(cc.FilePaths[i]); err != nil {
		// 			return peererrors.New(
		// 				peererrors.CodeLocalPeerError,
		// 				fmt.Sprintf("file path does not exist: %s", cc.FilePaths[i]),
		// 				ErrInvalidFilePath,
		// 				featureCapsule,
		// 			)
		// 		}
		// 	}
	}

	if hasLetter {
		letterFileInfo, err := cc.Letter.Stat()
		if err != nil {
			return err
		}

		log.Println(letterFileInfo.Name() == "", letterFileInfo.Size() > 0, "<<<< check has")

		if letterFileInfo.Name() == "" || letterFileInfo.Size() <= 0 {
			return peererrors.New(
				peererrors.CodeLocalPeerError,
				"letter must be a valid letter that has a name and content",
				nil,
				featureCapsule,
			)
		}
	}

	// Set threshold if not specified
	if cc.CapsuleMasterKeyRecoveryThreshold == 0 {
		cc.CapsuleMasterKeyRecoveryThreshold = calculateDefaultThreshold(
			len(cc.RemotePeerGuardians),
		)
	}

	// Validate threshold bounds
	if cc.CapsuleMasterKeyRecoveryThreshold < 2 {
		return peererrors.New(
			peererrors.CodeLocalPeerError,
			"recovery threshold must be at least 2",
			ErrInvalidCapsuleMasterKeyRecoveryThreshold,
			featureCapsule,
		)
	}

	if cc.CapsuleMasterKeyRecoveryThreshold > len(cc.RemotePeerGuardians) {
		return peererrors.New(
			peererrors.CodeLocalPeerError,
			"recovery threshold cannot exceed number of guardians",
			ErrInvalidCapsuleMasterKeyRecoveryThreshold,
			featureCapsule,
		)
	}

	return nil
}

func (cc *CreateCapsuleDTO) GetNumOfFiles() (int, error) {
	numOfFiles := len(cc.FilePaths)

	letterFileInfo, err := cc.Letter.Stat()
	if err != nil {
		return 0, peererrors.New(
			peererrors.CodeInternalPeerError,
			"failed to get file info in capsule dto",
			err,
			featureCapsule,
		)
	}

	if cc.Letter != nil && (letterFileInfo.Name() != "" || letterFileInfo.Size() > 0) {
		numOfFiles += 1
	}

	return numOfFiles, nil
}

func (cc *CreateCapsuleDTO) GetTotalSize() (int64, error) {
	totalSize := int64(0)

	letterFileInfo, err := cc.Letter.Stat()
	if err != nil {
		return 0, peererrors.New(
			peererrors.CodeInternalPeerError,
			"failed to get file info in capsule dto",
			err,
			featureCapsule,
		)
	}

	if cc.Letter != nil && (letterFileInfo.Name() != "" || letterFileInfo.Size() > 0) {
		totalSize += letterFileInfo.Size()
	}

	for i := range cc.FilePaths {
		fileInfo, err := os.Stat(cc.FilePaths[i])
		if err != nil {
			return 0, peererrors.New(
				peererrors.CodeInternalPeerError,
				fmt.Sprintf(
					"failed to get file info in capsule dto for size for this file: %s",
					cc.FilePaths[i],
				),
				err,
				featureCapsule,
			)
		}

		// if fileInfo.Name() != "" || fileInfo.Size() > 0 {
		// }
		totalSize += fileInfo.Size()
	}

	return totalSize, nil
}

// outgoing
